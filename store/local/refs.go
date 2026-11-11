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
	return s.scanReferences(data, nil)
}

// scanReferences matches data against registered paths plus extra candidates.
// extra holds paths that are not valid yet: the output being scanned and its
// sibling outputs. Nix's referenceable set is the input closure plus every
// output of the derivation, including the output itself.
func (s *Store) scanReferences(data []byte, extra []string) ([]string, error) {
	candidates, err := s.listValidPaths()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(candidates)+len(extra))
	for _, c := range candidates {
		seen[c] = struct{}{}
	}
	for _, c := range extra {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		candidates = append(candidates, c)
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

// ScanPathReferences scans a store object (file or directory tree) for references
// to registered paths and to logicalPath itself.
func (s *Store) ScanPathReferences(logicalPath string) ([]string, error) {
	return s.ScanPathReferencesAmong(logicalPath, []string{logicalPath})
}

// ScanPathReferencesAmong scans logicalPath for references to registered paths
// and to also (the output itself and its sibling outputs).
func (s *Store) ScanPathReferencesAmong(logicalPath string, also []string) ([]string, error) {
	phys := PhysicalPath(s.Root, logicalPath)
	info, err := os.Stat(phys)
	if err != nil {
		return nil, err
	}
	extra := make([]string, 0, len(also)+1)
	extra = append(extra, logicalPath)
	extra = append(extra, also...)
	if !info.IsDir() {
		data, err := os.ReadFile(phys)
		if err != nil {
			return nil, err
		}
		return s.scanReferences(data, extra)
	}
	merged := map[string]struct{}{}
	err = filepath.WalkDir(phys, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		refs, err := s.scanReferences(data, extra)
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
