// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package local implements the on-disk Nix store (Phase 4b).
package local

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/connerohnesorge/goonix/store"

	_ "modernc.org/sqlite"
)

// Store is a content-addressed store with SQLite metadata.
type Store struct {
	Root   string
	pathCfg store.Config
	DB     *sql.DB
}

// OpenStore opens or creates a store at physical root (logical paths use /nix/store unless configured).
func OpenStore(physicalRoot string) (*Store, error) {
	return OpenStoreWithConfig(physicalRoot, store.Config{})
}

// OpenStoreWithConfig opens a store with a custom logical store directory prefix.
func OpenStoreWithConfig(physicalRoot string, cfg store.Config) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(physicalRoot, "store"), 0o755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(physicalRoot, "db.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{Root: physicalRoot, pathCfg: cfg, DB: db}
	if err := s.initSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) pathConfig() store.Config { return s.pathCfg }

// Config returns the logical store path configuration.
func (s *Store) Config() store.Config { return s.pathCfg }

// StoreDir is the logical prefix for store paths (NIX_STORE_DIR style).
func (s *Store) StoreDir() string { return s.pathCfg.LogicalStoreDir() }

func (s *Store) initSchema() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ValidPaths (path TEXT PRIMARY KEY, narHash TEXT, narSize INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS Refs (referrer TEXT, reference TEXT, PRIMARY KEY(referrer, reference))`,
		`CREATE TABLE IF NOT EXISTS DerivationOutputs (drvPath TEXT, id TEXT, path TEXT, PRIMARY KEY(drvPath, id))`,
		`CREATE TABLE IF NOT EXISTS GcRoots (root TEXT PRIMARY KEY, path TEXT NOT NULL)`,
	}
	for _, q := range stmts {
		if _, err := s.DB.Exec(q); err != nil {
			return err
		}
	}
	// Migrate older DBs created before narHash columns existed.
	for _, mig := range []string{
		`ALTER TABLE ValidPaths ADD COLUMN narHash TEXT`,
		`ALTER TABLE ValidPaths ADD COLUMN narSize INTEGER NOT NULL DEFAULT 0`,
	} {
		_, _ = s.DB.Exec(mig)
	}
	return nil
}

// AddTextPath registers a text path already written under Root (legacy helper).
func (s *Store) AddTextPath(logicalPath string, narHash string) error {
	return s.registerPath(logicalPath, narHash, 0, nil)
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

// IsValidPath reports whether logicalPath is registered (alias for method on add.go).
func (s *Store) isValidPathQuery(logicalPath string) (bool, error) {
	var one int
	err := s.DB.QueryRow(`SELECT 1 FROM ValidPaths WHERE path = ?`, logicalPath).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

var _ = fmt.Sprintf
