// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package local implements the on-disk Nix store (Phase 4b).
package local

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store is a content-addressed store with SQLite metadata.
type Store struct {
	Root string
	DB   *sql.DB
}

// OpenStore opens or creates a store at physical root (logical paths remain /nix/store/…).
func OpenStore(physicalRoot string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(physicalRoot, "store"), 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(physicalRoot, "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{Root: physicalRoot, DB: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) initSchema() error {
	_, err := s.DB.Exec(`
CREATE TABLE IF NOT EXISTS ValidPaths (path TEXT PRIMARY KEY);
CREATE TABLE IF NOT EXISTS Refs (referrer TEXT, reference TEXT, PRIMARY KEY(referrer, reference));
CREATE TABLE IF NOT EXISTS DerivationOutputs (drvPath TEXT, id TEXT, path TEXT, PRIMARY KEY(drvPath, id));
`)
	return err
}

// AddTextPath registers a text path already written under Root.
func (s *Store) AddTextPath(logicalPath string, narHash string) error {
	_, err := s.DB.Exec(`INSERT OR IGNORE INTO ValidPaths(path) VALUES (?)`, logicalPath)
	return err
}

// WriteDerivation writes a .drv file using store computation helpers.
func (s *Store) WriteDerivation(content []byte, drvPath string) error {
	phys := PhysicalPath(s.Root, drvPath)
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		return err
	}
	tmp := phys + ".tmp"
	if err := os.WriteFile(tmp, content, 0o444); err != nil {
		return err
	}
	if err := os.Rename(tmp, phys); err != nil {
		return err
	}
	return s.AddTextPath(drvPath, "")
}

// Close closes the metadata database.
func (s *Store) Close() error {
	if s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// PhysicalPath maps a logical store path to the physical layout.
func PhysicalPath(root, logical string) string {
	if root == "" {
		return logical
	}
	base := filepath.Base(logical)
	return filepath.Join(root, "store", base)
}

var _ = fmt.Sprintf
