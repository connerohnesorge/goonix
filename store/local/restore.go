// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gonixnar "github.com/nix-community/go-nix/pkg/nar"

	"github.com/connerohnesorge/goonix/store"
)

// unpackNAR restores the NAR in r at dest. Regular files and directories are
// written as they arrive. Symlinks are created afterwards, when their targets
// exist, because a Windows link is a file link or a directory link depending
// on what it points at. On a case-insensitive store a name that collides with
// a sibling gets the case-hack suffix. Everything ends up read-only.
func unpackNAR(r io.Reader, dest string) error {
	nr, err := gonixnar.NewReader(r)
	if err != nil {
		return err
	}
	defer nr.Close()
	u := &narRestorer{root: dest, names: map[string]map[string]int{}, disk: map[string]string{"/": dest}}
	for {
		hdr, err := nr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := u.entry(nr, hdr); err != nil {
			return err
		}
	}
	if err := u.createLinks(); err != nil {
		return err
	}
	return finalizeNARModes(dest)
}

type pendingLink struct {
	target, path string
}

type narRestorer struct {
	root string
	// names counts, per parent directory, the entries whose names fold to
	// the same key, for the case hack.
	names map[string]map[string]int
	// disk maps a NAR path to the path it was written at.
	disk  map[string]string
	links []pendingLink
}

// diskPath returns where the NAR entry at narPath goes on disk.
func (u *narRestorer) diskPath(narPath string) (string, error) {
	if narPath == "/" {
		return u.root, nil
	}
	parentNar, name := narSplit(narPath)
	if err := checkEntryName(name); err != nil {
		return "", err
	}
	parent, ok := u.disk[parentNar]
	if !ok {
		return "", fmt.Errorf("NAR entry '%s' has no parent directory", narPath)
	}
	diskName := name
	if store.CaseInsensitiveStore {
		key := strings.ToUpper(name)
		seen := u.names[parent]
		if seen == nil {
			seen = map[string]int{}
			u.names[parent] = seen
		}
		if n := seen[key]; n > 0 {
			diskName = fmt.Sprintf("%s%s%d", name, store.CaseHackSuffix, n)
		}
		seen[key]++
	}
	p := filepath.Join(parent, diskName)
	u.disk[narPath] = p
	return p, nil
}

func narSplit(p string) (parent, name string) {
	i := strings.LastIndex(p, "/")
	parent = p[:i]
	if parent == "" {
		parent = "/"
	}
	return parent, p[i+1:]
}

func (u *narRestorer) entry(nr *gonixnar.Reader, hdr *gonixnar.Header) error {
	target, err := u.diskPath(hdr.Path)
	if err != nil {
		return err
	}
	switch hdr.Type {
	case gonixnar.TypeDirectory:
		return os.MkdirAll(target, 0o755)
	case gonixnar.TypeSymlink:
		u.links = append(u.links, pendingLink{target: hdr.LinkTarget, path: target})
		return nil
	case gonixnar.TypeRegular:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, nr)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if hdr.Executable {
			return store.MarkExecutable(target)
		}
		return nil
	default:
		return fmt.Errorf("unknown nar node type %q", hdr.Type)
	}
}

// createLinks creates the pending symlinks. A link whose target passes
// through another pending link waits for it, so the target's kind is known
// when the link is made; links left in a cycle are created as they are.
func (u *narRestorer) createLinks() error {
	pending := u.links
	sort.Slice(pending, func(i, j int) bool { return pending[i].path < pending[j].path })
	for len(pending) > 0 {
		var later []pendingLink
		for _, l := range pending {
			if u.waitsOn(l, pending) {
				later = append(later, l)
				continue
			}
			if err := makeLink(l); err != nil {
				return err
			}
		}
		if len(later) == len(pending) {
			for _, l := range later {
				if err := makeLink(l); err != nil {
					return err
				}
			}
			return nil
		}
		pending = later
	}
	return nil
}

func (u *narRestorer) waitsOn(l pendingLink, pending []pendingLink) bool {
	resolved := l.target
	if !filepath.IsAbs(filepath.FromSlash(resolved)) {
		resolved = filepath.Join(filepath.Dir(l.path), filepath.FromSlash(resolved))
	}
	resolved = filepath.Clean(resolved)
	for _, o := range pending {
		if o.path == l.path {
			continue
		}
		if resolved == o.path || strings.HasPrefix(resolved, o.path+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func makeLink(l pendingLink) error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	if err := os.Symlink(l.target, l.path); err != nil {
		return symlinkError(l, err)
	}
	return nil
}

// finalizeNARModes makes the restored tree read-only: 0555 directories,
// 0555 or 0444 files. Symlinks are skipped; chmod would follow them.
func finalizeNARModes(root string) error {
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if d.IsDir() {
			dirs = append(dirs, p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode().Perm()&0o111 != 0 {
			return os.Chmod(p, 0o555)
		}
		return os.Chmod(p, 0o444)
	})
	if err != nil {
		return err
	}
	// Children first: a read-only directory still lists, but the walk above
	// must finish before any directory is locked.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := os.Chmod(dirs[i], 0o555); err != nil {
			return err
		}
	}
	return nil
}

// MakeReadOnly gives a store object Nix's permissions: directories 0555,
// files 0555 when executable and 0444 otherwise. Symlinks are left alone.
func MakeReadOnly(path string) error { return finalizeNARModes(path) }
