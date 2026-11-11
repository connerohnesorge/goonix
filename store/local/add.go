// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	gonixnar "github.com/nix-community/go-nix/pkg/nar"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"

	"github.com/connerohnesorge/goonix/store"
)

// AddToStoreText writes a text object and registers it in the DB.
func (s *Store) AddToStoreText(name, contents string) (logicalPath string, err error) {
	logicalPath, err = s.pathConfig().StorePathForText(name, contents)
	if err != nil {
		return "", err
	}
	phys := PhysicalPath(s.Root, logicalPath)
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(phys, []byte(contents), 0o444); err != nil {
		return "", err
	}
	refs, err := s.ScanReferences([]byte(contents))
	if err != nil {
		return "", err
	}
	return logicalPath, s.registerPath(logicalPath, "", 0, refs)
}

// AddToStoreFlat copies a single file into the store (flat NAR fingerprint).
func (s *Store) AddToStoreFlat(sourceFile, name string) (logicalPath string, err error) {
	logicalPath, err = s.pathConfig().StorePathForFilesystemPath(sourceFile, name)
	if err != nil {
		return "", err
	}
	phys := PhysicalPath(s.Root, logicalPath)
	if err := copyFile(sourceFile, phys); err != nil {
		return "", err
	}
	narHash, narSize, err := FlatNarHashAndSize(sourceFile)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := gonixnar.DumpPath(&buf, sourceFile); err != nil {
		return "", err
	}
	refs, err := s.ScanReferences(buf.Bytes())
	if err != nil {
		return "", err
	}
	return logicalPath, s.registerPath(logicalPath, narHash, narSize, refs)
}

// AddToStoreRecursive copies a directory tree into the store.
func (s *Store) AddToStoreRecursive(sourceDir, name string) (logicalPath string, err error) {
	logicalPath, err = s.pathConfig().StorePathForFilesystemPath(sourceDir, name)
	if err != nil {
		return "", err
	}
	phys := PhysicalPath(s.Root, logicalPath)
	if err := store.CopyPathToStore(sourceDir, phys); err != nil {
		return "", err
	}
	narHash, narSize, err := NarHashAndSize(phys)
	if err != nil {
		return "", err
	}
	refs, err := s.ScanPathReferences(logicalPath)
	if err != nil {
		return "", err
	}
	return logicalPath, s.registerPath(logicalPath, narHash, narSize, refs)
}

// ImportNAR streams the NAR in r into logicalPath. It hashes the NAR while
// unpacking it into a temporary directory beside the store object, and only
// moves it into place and registers it when the hash equals expectNarHash
// (if given). Memory use does not grow with the NAR size.
func (s *Store) ImportNAR(logicalPath string, r io.Reader, expectNarHash string) error {
	phys := PhysicalPath(s.Root, logicalPath)
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(filepath.Dir(phys), ".import-")
	if err != nil {
		return err
	}
	defer RemoveStoreTree(tmpDir)
	h := sha256.New()
	counted := &countingReader{r: io.TeeReader(r, h)}
	tmp := filepath.Join(tmpDir, "nar")
	if err := unpackNAR(counted, tmp); err != nil {
		return err
	}
	// Hash any bytes the NAR reader left unread, so trailing data fails the
	// hash check instead of being ignored.
	if _, err := io.Copy(io.Discard, counted); err != nil {
		return err
	}
	got := nixbase32.EncodeToString(h.Sum(nil))
	if expectNarHash != "" && got != expectNarHash {
		return fmt.Errorf("nar hash mismatch: got %s want %s", got, expectNarHash)
	}
	if err := RemoveStoreTree(phys); err != nil {
		return err
	}
	// Moving a directory updates its ".." entry, which needs write
	// permission on the directory itself; restore 0555 afterwards.
	fi, err := os.Lstat(tmp)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.Chmod(tmp, 0o755); err != nil {
			return err
		}
	}
	if err := os.Rename(tmp, phys); err != nil {
		return err
	}
	if fi.IsDir() {
		if err := os.Chmod(phys, 0o555); err != nil {
			return err
		}
	}
	refs, err := s.ScanPathReferences(logicalPath)
	if err != nil {
		return err
	}
	return s.registerPath(logicalPath, got, counted.n, refs)
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// RemoveStoreTree deletes a store object. Store directories are read-only
// (0555), and unlinking an entry needs write permission on its directory,
// so it makes the directories writable first. A missing path is not an error.
func RemoveStoreTree(path string) error {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return nil
	}
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	return os.RemoveAll(path)
}

func (s *Store) registerPath(logicalPath, narHash string, narSize int64, refs []string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO ValidPaths(path, narHash, narSize) VALUES (?, ?, ?)`, logicalPath, narHash, narSize); err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO Refs(referrer, reference) VALUES (?, ?)`, logicalPath, ref); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// IsValidPath reports whether logicalPath is registered.
func (s *Store) IsValidPath(logicalPath string) (bool, error) {
	return s.isValidPathQuery(logicalPath)
}

// PathMeta is the registered NAR metadata Nix puts in structured-attrs path info.
type PathMeta struct {
	NarHash string
	NarSize int64
	Refs    []string
}

// QueryPathMeta returns the registered NAR hash (nixbase32, no prefix), size, and references.
func (s *Store) QueryPathMeta(logicalPath string) (PathMeta, error) {
	var meta PathMeta
	err := s.DB.QueryRow(`SELECT narHash, narSize FROM ValidPaths WHERE path = ?`, logicalPath).Scan(&meta.NarHash, &meta.NarSize)
	if err != nil {
		return PathMeta{}, err
	}
	meta.Refs, err = s.QueryReferences(logicalPath)
	if err != nil {
		return PathMeta{}, err
	}
	return meta, nil
}

// QueryReferences returns direct references of logicalPath.
func (s *Store) QueryReferences(logicalPath string) ([]string, error) {
	rows, err := s.DB.Query(`SELECT reference FROM Refs WHERE referrer = ? ORDER BY reference`, logicalPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		out = append(out, ref)
	}
	return out, rows.Err()
}

// RecordDerivationOutput links a built output path to its derivation.
func (s *Store) RecordDerivationOutput(drvPath, outputID, outPath string) error {
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO DerivationOutputs(drvPath, id, path) VALUES (?, ?, ?)`, drvPath, outputID, outPath)
	return err
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o444)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func unpackNAR(r io.Reader, dest string) error {
	nr, err := gonixnar.NewReader(r)
	if err != nil {
		return err
	}
	defer nr.Close()

	first, err := nr.Next()
	if err != nil {
		return err
	}
	// Single regular file at "/" → flat store object (dest is the file path).
	if first.Path == "/" && first.Type == gonixnar.TypeRegular {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o444)
		if first.Executable {
			mode = 0o555
		}
		f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, nr)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	if err := writeNAREntry(nr, first, dest); err != nil {
		return err
	}
	for {
		hdr, err := nr.Next()
		if err == io.EOF {
			return finalizeNARModes(dest)
		}
		if err != nil {
			return err
		}
		if err := writeNAREntry(nr, hdr, dest); err != nil {
			return err
		}
	}
}

func finalizeNARModes(root string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(p, 0o555)
		}
		mode := info.Mode().Perm()
		if mode&0o111 != 0 {
			return os.Chmod(p, 0o555)
		}
		return os.Chmod(p, 0o444)
	})
}

func writeNAREntry(nr *gonixnar.Reader, hdr *gonixnar.Header, destRoot string) error {
	rel := hdr.Path
	if rel == "/" {
		rel = ""
	}
	target := filepath.Join(destRoot, filepath.FromSlash(rel))
	switch hdr.Type {
	case gonixnar.TypeDirectory:
		return os.MkdirAll(target, 0o755)
	case gonixnar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Symlink(hdr.LinkTarget, target)
	case gonixnar.TypeRegular:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o444)
		if hdr.Executable {
			mode = 0o555
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		_, err = io.Copy(f, nr)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	default:
		return fmt.Errorf("unknown nar node type %q", hdr.Type)
	}
}

// VerifyFlatOutput checks a fixed-output flat hash against a built file (raw content, not NAR).
func VerifyFlatOutput(filePath, wantBase16, algo string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	if algo == "" {
		algo = "sha256"
	}
	var gotBase16 string
	switch algo {
	case "md5":
		sum := md5.Sum(data)
		gotBase16 = hex.EncodeToString(sum[:])
	case "sha256":
		sum := sha256.Sum256(data)
		gotBase16 = hex.EncodeToString(sum[:])
	case "sha1":
		sum := sha1.Sum(data)
		gotBase16 = hex.EncodeToString(sum[:])
	case "sha512":
		sum := sha512.Sum512(data)
		gotBase16 = hex.EncodeToString(sum[:])
	default:
		return fmt.Errorf("unknown hash algorithm '%s', expect 'md5', 'sha1', 'sha256', or 'sha512'", algo)
	}
	if gotBase16 != wantBase16 {
		return &OutputHashMismatch{GotBase16: gotBase16}
	}
	return nil
}

// OutputHashMismatch is a fixed-output digest that is not the one the derivation specified.
type OutputHashMismatch struct {
	GotBase16 string
}

func (e *OutputHashMismatch) Error() string {
	return "hash mismatch"
}

// VerifyRecursiveOutput checks a fixed-output recursive hash.
func VerifyRecursiveOutput(path, wantBase16 string) error {
	gotHash, _, err := NarHashAndSize(path)
	if err != nil {
		return err
	}
	raw, err := nixhash.ParseAny("sha256:"+gotHash, nil)
	if err != nil {
		return err
	}
	gotBase16 := raw.Hash.Format(nixhash.Base16, false)
	if gotBase16 != wantBase16 {
		return &OutputHashMismatch{GotBase16: gotBase16}
	}
	return nil
}
