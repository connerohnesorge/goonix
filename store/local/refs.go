// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/nix-community/go-nix/pkg/storepath/references"
)

// ScanReferences finds store path references in data by matching known valid paths.
func (s *Store) ScanReferences(data []byte) ([]string, error) {
	candidates, err := s.listValidPaths()
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	scanner, err := references.NewReferenceScanner(candidates)
	if err != nil {
		return nil, err
	}
	if _, err := scanner.Write(data); err != nil {
		return nil, err
	}
	return scanner.References(), nil
}

// ScanPathReferences scans a store object (file or directory tree) for references.
func (s *Store) ScanPathReferences(logicalPath string) ([]string, error) {
	phys := PhysicalPath(s.Root, logicalPath)
	info, err := os.Stat(phys)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(phys)
		if err != nil {
			return nil, err
		}
		return s.ScanReferences(data)
	}
	merged := map[string]struct{}{}
	err = filepath.WalkDir(phys, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		refs, err := s.ScanReferences(data)
		if err != nil {
			return err
		}
		for _, r := range refs {
			merged[r] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(merged))
	for r := range merged {
		out = append(out, r)
	}
	sort.Strings(out)
	return out, nil
}

// RegisterReferences inserts Refs rows for referrer -> references.
func (s *Store) RegisterReferences(referrer string, refs []string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, ref := range refs {
		if ref == referrer {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO Refs(referrer, reference) VALUES (?, ?)`, referrer, ref); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) listValidPaths() ([]string, error) {
	rows, err := s.DB.Query(`SELECT path FROM ValidPaths`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
