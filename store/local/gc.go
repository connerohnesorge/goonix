// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"fmt"
	"path/filepath"

	"database/sql"
	"errors"
)

// AddGCRoot records a GC root stub entry (full GC not implemented).
func (s *Store) AddGCRoot(name, logicalPath string) error {
	if name == "" {
		return fmt.Errorf("empty gc root name")
	}
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO GcRoots(root, path) VALUES (?, ?)`, name, logicalPath)
	return err
}

// GCRootPath returns the store path pinned by name, if any.
func (s *Store) GCRootPath(name string) (string, bool, error) {
	var p string
	err := s.DB.QueryRow(`SELECT path FROM GcRoots WHERE root = ?`, name).Scan(&p)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return p, true, nil
}

// PhysicalGCRootsDir returns the directory used for per-root symlinks (stub layout).
func (s *Store) PhysicalGCRootsDir() string {
	return filepath.Join(s.Root, "gcroots")
}

var _ = fmt.Sprintf
