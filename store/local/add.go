// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	gonixnar "github.com/nix-community/go-nix/pkg/nar"
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
	var buf bytesBuffer
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

// ImportNAR unpacks verified NAR bytes into logicalPath and registers metadata.
func (s *Store) ImportNAR(logicalPath string, r io.Reader, expectNarHash string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	got, narSize, err := HashNARStream(bytesNewReader(data))
	if err != nil {
		return err
	}
	if expectNarHash != "" && got != expectNarHash {
		return fmt.Errorf("nar hash mismatch: got %s want %s", got, expectNarHash)
	}
	phys := PhysicalPath(s.Root, logicalPath)
	if err := os.RemoveAll(phys); err != nil {
		return err
	}
	if err := unpackNAR(bytesNewReader(data), phys); err != nil {
		return err
	}
	narHash := got
	if expectNarHash != "" {
		narHash = expectNarHash
	}
	refs, err := s.ScanPathReferences(logicalPath)
	if err != nil {
		return err
	}
	return s.registerPath(logicalPath, narHash, narSize, refs)
}

func bytesNewReader(b []byte) *bytesReader { return &bytesReader{b: b} }

type bytesReader struct {
	b []byte
	i int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
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

type bytesBuffer struct {
	b []byte
}

func (b *bytesBuffer) Write(p []byte) (int, error) {
	b.b = append(b.b, p...)
	return len(p), nil
}

func (b *bytesBuffer) Bytes() []byte { return b.b }

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

	if err := os.MkdirAll(dest, 0o555); err != nil {
		return err
	}
	if err := writeNAREntry(nr, first, dest); err != nil {
		return err
	}
	for {
		hdr, err := nr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := writeNAREntry(nr, hdr, dest); err != nil {
			return err
		}
	}
}

func writeNAREntry(nr *gonixnar.Reader, hdr *gonixnar.Header, destRoot string) error {
	rel := hdr.Path
	if rel == "/" {
		rel = ""
	}
	target := filepath.Join(destRoot, filepath.FromSlash(rel))
	switch hdr.Type {
	case gonixnar.TypeDirectory:
		return os.MkdirAll(target, 0o555)
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

// VerifyFlatOutput checks a fixed-output flat hash against a built file.
func VerifyFlatOutput(filePath, wantBase16 string) error {
	gotHash, _, err := FlatNarHashAndSize(filePath)
	if err != nil {
		return err
	}
	raw, err := nixhash.ParseAny("sha256:"+gotHash, nil)
	if err != nil {
		return err
	}
	gotBase16 := raw.Hash.Format(nixhash.Base16, false)
	if gotBase16 != wantBase16 {
		return fmt.Errorf("fixed-output hash mismatch: got %s want %s", gotBase16, wantBase16)
	}
	return nil
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
		return fmt.Errorf("fixed-output hash mismatch: got %s want %s", gotBase16, wantBase16)
	}
	return nil
}
