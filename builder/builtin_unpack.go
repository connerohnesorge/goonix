// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/klauspost/compress/zstd"
	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

// builtin:unpack is NovaNix's in-process builder that extracts the archives
// in `srcs`, in order, into one `out` directory. NovaNix's Windows seeds use
// it to merge pinned MSYS2 and MinGW-w64 packages into a toolchain root. The
// output is a plain tree: links in the archives become copies, an owner-execute
// bit becomes the executable flag, and root-level dot entries (.PKGINFO,
// .BUILDINFO, .MTREE, .INSTALL) are left out.
const builtinUnpack = "builtin:unpack"

const (
	unpackMaxBytes   = int64(4) * 1024 * 1024 * 1024
	unpackMaxEntries = 1000000
)

func unpackErr(format string, args ...any) error {
	return fmt.Errorf("builtin:unpack: "+format, args...)
}

func (b *Builder) buildBuiltinUnpack(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string) error {
	srcsText, ok := d.Env["srcs"]
	if !ok {
		return unpackErr("derivation has no 'srcs'")
	}
	srcs := strings.Fields(srcsText)
	if len(srcs) == 0 {
		return unpackErr("'srcs' is empty")
	}
	outPath, ok := outPaths["out"]
	if !ok {
		return unpackErr("derivation defines no 'out' output")
	}
	storeDir := b.Store.Config().LogicalStoreDir()
	archives := make([]string, 0, len(srcs))
	for _, s := range srcs {
		if !strings.HasPrefix(s, storeDir+"/") || strings.Contains(strings.TrimPrefix(s, storeDir+"/"), "/") {
			return unpackErr("'srcs' entry is not a store path: %s", s)
		}
		archives = append(archives, local.PhysicalPath(b.Store.Root, s))
	}
	phys := local.PhysicalPath(b.Store.Root, outPath)
	if err := local.RemoveStoreTree(phys); err != nil {
		return err
	}
	if err := os.MkdirAll(phys, 0o755); err != nil {
		return err
	}
	u := &unpacker{root: phys}
	for _, a := range archives {
		if err := u.extract(a); err != nil {
			_ = local.RemoveStoreTree(phys)
			return unpackErr("%s", err)
		}
	}
	if err := local.MakeReadOnly(phys); err != nil {
		return err
	}
	refs, err := b.Store.ScanPathReferences(outPath)
	if err != nil {
		return err
	}
	if err := b.Store.RecordDerivationOutput(drvPath, "out", outPath); err != nil {
		return err
	}
	return b.registerBuilt(outPath, phys, refs)
}

type unpacker struct {
	root    string
	bytes   int64
	entries int
}

func (u *unpacker) charge(n int64) error {
	u.entries++
	u.bytes += n
	if u.entries > unpackMaxEntries {
		return fmt.Errorf("archive has more than %d entries", unpackMaxEntries)
	}
	if u.bytes > unpackMaxBytes {
		return fmt.Errorf("archive expands to more than %d bytes", unpackMaxBytes)
	}
	return nil
}

func (u *unpacker) extract(archive string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	var r io.Reader
	lower := strings.ToLower(archive)
	switch {
	case strings.HasSuffix(lower, ".tar.zst"):
		dec, err := zstd.NewReader(f)
		if err != nil {
			return fmt.Errorf("%s: %w", archive, err)
		}
		defer dec.Close()
		r = dec
	case strings.HasSuffix(lower, ".tar.gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("%s: %w", archive, err)
		}
		defer gz.Close()
		r = gz
	case strings.HasSuffix(lower, ".tar"):
		r = f
	default:
		return fmt.Errorf("unsupported archive format: %s", archive) // stub-scan: exempt (NovaNix's message)
	}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("malformed archive: %s: %w", archive, err)
		}
		if err := u.entry(tr, hdr); err != nil {
			return err
		}
	}
}

// entryComponents splits an archive path into its components, dropping "."
// and refusing anything that would land outside the output.
func entryComponents(raw string) ([]string, error) {
	if strings.HasPrefix(raw, "/") || (runtime.GOOS == "windows" && (strings.HasPrefix(raw, `\`) || filepath.VolumeName(raw) != "")) {
		return nil, fmt.Errorf("absolute entry path: %s", raw)
	}
	split := func(r rune) bool { return r == '/' || (runtime.GOOS == "windows" && r == '\\') }
	var out []string
	for _, c := range strings.FieldsFunc(raw, split) {
		switch {
		case c == ".":
			continue
		case c == "..":
			return nil, fmt.Errorf("entry path escapes archive root: %s", raw)
		case strings.Contains(c, ":"):
			return nil, fmt.Errorf("entry path contains ':': %s", raw)
		}
		out = append(out, c)
	}
	return out, nil
}

func (u *unpacker) dest(comps []string) string {
	return filepath.Join(append([]string{u.root}, comps...)...)
}

// fresh refuses to write over anything already extracted, from this archive
// or an earlier one.
func fresh(dest string) error {
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("file collision between archives: %s", dest)
	}
	return nil
}

func (u *unpacker) entry(tr *tar.Reader, hdr *tar.Header) error {
	switch hdr.Typeflag {
	case tar.TypeXGlobalHeader, tar.TypeXHeader:
		return nil
	}
	comps, err := entryComponents(hdr.Name)
	if err != nil {
		return err
	}
	if len(comps) == 0 {
		return nil
	}
	// Package metadata: a dot entry at the archive root.
	if len(comps) == 1 && strings.HasPrefix(comps[0], ".") {
		return nil
	}
	dest := u.dest(comps)
	switch hdr.Typeflag {
	case tar.TypeDir:
		if err := u.charge(0); err != nil {
			return err
		}
		return os.MkdirAll(dest, 0o755)
	case tar.TypeReg, tar.TypeRegA:
		if err := u.charge(hdr.Size); err != nil {
			return err
		}
		if err := fresh(dest); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, tr)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		if hdr.Mode&0o100 != 0 {
			return store.MarkExecutable(dest)
		}
		return nil
	case tar.TypeSymlink:
		target, err := resolveLinkTarget(comps, hdr.Linkname, hdr.Name)
		if err != nil {
			return err
		}
		return u.copyLink("symlink", hdr.Name, hdr.Linkname, u.dest(target), dest)
	case tar.TypeLink:
		target, err := entryComponents(hdr.Linkname)
		if err != nil {
			return err
		}
		return u.copyLink("hard link", hdr.Name, hdr.Linkname, u.dest(target), dest)
	case tar.TypeChar:
		return fmt.Errorf("unsupported character device entry: %s", hdr.Name) // stub-scan: exempt
	case tar.TypeBlock:
		return fmt.Errorf("unsupported block device entry: %s", hdr.Name) // stub-scan: exempt
	case tar.TypeFifo:
		return fmt.Errorf("unsupported FIFO entry: %s", hdr.Name) // stub-scan: exempt
	default:
		return fmt.Errorf("unsupported tar entry type '%c': %s", hdr.Typeflag, hdr.Name) // stub-scan: exempt
	}
}

// resolveLinkTarget resolves a symlink target against the link's directory,
// within the archive root.
func resolveLinkTarget(linkComps []string, target, name string) ([]string, error) {
	if strings.HasPrefix(target, "/") || (runtime.GOOS == "windows" && (strings.HasPrefix(target, `\`) || filepath.VolumeName(target) != "")) {
		return nil, fmt.Errorf("absolute symlink target: %s -> %s", name, target)
	}
	split := func(r rune) bool { return r == '/' || (runtime.GOOS == "windows" && r == '\\') }
	out := append([]string{}, linkComps[:len(linkComps)-1]...)
	for _, c := range strings.FieldsFunc(target, split) {
		switch {
		case c == ".":
		case c == "..":
			if len(out) == 0 {
				return nil, fmt.Errorf("symlink target escapes archive root: %s -> %s", name, target)
			}
			out = out[:len(out)-1]
		case strings.Contains(c, ":"):
			return nil, fmt.Errorf("symlink target contains ':': %s -> %s", name, target)
		default:
			out = append(out, c)
		}
	}
	return out, nil
}

// copyLink materialises a link as a copy of what it points at, which must
// already be extracted.
func (u *unpacker) copyLink(kind, name, linkname, from, to string) error {
	fi, err := os.Stat(from)
	if err != nil {
		return fmt.Errorf("%s target not present (links must follow their targets in the archive): %s -> %s", kind, name, linkname)
	}
	if err := fresh(to); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	if fi.IsDir() {
		if err := u.charge(0); err != nil {
			return err
		}
		return u.copyTree(from, to)
	}
	if err := u.charge(fi.Size()); err != nil {
		return err
	}
	return copyFileWithExec(from, to, fi)
}

func (u *unpacker) copyTree(from, to string) error {
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		src, dst := filepath.Join(from, e.Name()), filepath.Join(to, e.Name())
		fi, err := os.Stat(src)
		if err != nil {
			return err
		}
		if fi.IsDir() {
			if err := u.charge(0); err != nil {
				return err
			}
			if err := u.copyTree(src, dst); err != nil {
				return err
			}
			continue
		}
		if err := u.charge(fi.Size()); err != nil {
			return err
		}
		if err := copyFileWithExec(src, dst, fi); err != nil {
			return err
		}
	}
	return nil
}

func copyFileWithExec(from, to string, fi os.FileInfo) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if store.IsExecutable(from, fi) {
		return store.MarkExecutable(to)
	}
	return nil
}
