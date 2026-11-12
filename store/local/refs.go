// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"sort"

	"github.com/nix-community/go-nix/pkg/storepath/references"

	"github.com/connerohnesorge/goonix/store"
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
	scanner, err := s.newReferenceScanner(extra)
	if err != nil || scanner == nil {
		return nil, err
	}
	if _, err := scanner.Write(data); err != nil {
		return nil, err
	}
	return scanner.References(), nil
}

// newReferenceScanner returns a scanner over the registered paths plus extra,
// or nil when there is nothing to look for.
func (s *Store) newReferenceScanner(extra []string) (*references.ReferenceScanner, error) {
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
	return references.NewReferenceScanner(candidates)
}

// ScanPathReferences scans a store object (file or directory tree) for references
// to registered paths and to logicalPath itself.
func (s *Store) ScanPathReferences(logicalPath string) ([]string, error) {
	return s.ScanPathReferencesAmong(logicalPath, []string{logicalPath})
}

// ScanPathReferencesAmong scans logicalPath for references to registered paths
// and to also (the output itself and its sibling outputs). Like Nix, it scans
// the path's NAR serialisation, so symlink targets and file names count, and
// a top-level symlink output is read as a link, not followed.
func (s *Store) ScanPathReferencesAmong(logicalPath string, also []string) ([]string, error) {
	phys := PhysicalPath(s.Root, logicalPath)
	extra := make([]string, 0, len(also)+1)
	extra = append(extra, logicalPath)
	extra = append(extra, also...)
	scanner, err := s.newReferenceScanner(extra)
	if err != nil {
		return nil, err
	}
	if scanner == nil {
		return nil, nil
	}
	if err := store.DumpPath(scanner, phys); err != nil {
		return nil, err
	}
	out := scanner.References()
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
