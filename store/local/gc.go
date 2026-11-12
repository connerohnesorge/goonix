// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixhash"

	"github.com/connerohnesorge/goonix/store"
)

// AddGCRoot records a direct GC root named name that points at logicalPath.
// The root is a symlink under the store's gcroots directory, which CollectGarbage reads.
func (s *Store) AddGCRoot(name, logicalPath string) error {
	if name == "" || name == "auto" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid gc root name %q", name)
	}
	dir := s.PhysicalGCRootsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	link := filepath.Join(dir, name)
	if err := replaceSymlink(link, logicalPath); err != nil {
		return err
	}
	_, err := s.DB.Exec(`INSERT OR REPLACE INTO GcRoots(root, path) VALUES (?, ?)`, name, logicalPath)
	return err
}

// GCRootPath returns the store path pinned by a direct root name, if any.
func (s *Store) GCRootPath(name string) (string, bool, error) {
	link := filepath.Join(s.PhysicalGCRootsDir(), name)
	if target, err := store.ReadLinkTarget(link); err == nil {
		return target, true, nil
	}
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

// PhysicalGCRootsDir is the directory of direct GC roots (Nix's gcroots).
func (s *Store) PhysicalGCRootsDir() string {
	return filepath.Join(s.Root, "gcroots")
}

// AddIndirectRoot registers a weak root whose target is the user-facing symlink path.
// The link lives at gcroots/auto/<nix32(sha1(path))>, matching Nix 2.24.9.
// CollectGarbage removes the auto link when that symlink disappears.
func (s *Store) AddIndirectRoot(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	name, err := nix32SHA1(abs)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.PhysicalGCRootsDir(), "auto")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return replaceSymlink(filepath.Join(dir, name), abs)
}

// AddPermRoot creates a user-facing symlink to logicalPath and an indirect root for it.
func (s *Store) AddPermRoot(logicalPath, gcRoot string) error {
	abs, err := filepath.Abs(gcRoot)
	if err != nil {
		return err
	}
	if s.inStore(abs) {
		return fmt.Errorf("creating a garbage collector root (%s) in the Nix store is forbidden (are you running nix-build inside the store?)", abs)
	}
	if fi, err := os.Lstat(abs); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("cannot create symlink '%s'; already exists", abs)
		}
		target, err := store.ReadLinkTarget(abs)
		if err != nil || !s.inStore(target) {
			return fmt.Errorf("cannot create symlink '%s'; already exists", abs)
		}
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := replaceSymlink(abs, logicalPath); err != nil {
		return err
	}
	return s.AddIndirectRoot(abs)
}

// CollectGarbage deletes valid paths that are not reachable from a GC root.
// Roots are direct symlinks under gcroots and profiles, plus indirect auto roots.
// A path stays when a kept path references it.
//
// It holds gc.lock exclusively, so it waits for running builds (which hold
// it shared, see LockForBuild) and never deletes paths they use.
func (s *Store) CollectGarbage() (deleted []string, err error) {
	unlock, err := s.lockGC(true)
	if err != nil {
		return nil, err
	}
	defer unlock()
	roots, err := s.findGCRoots()
	if err != nil {
		return nil, err
	}
	kept := map[string]bool{}
	stack := make([]string, 0, len(roots))
	for p := range roots {
		stack = append(stack, p)
	}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if kept[p] {
			continue
		}
		ok, err := s.IsValidPath(p)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		kept[p] = true
		refs, err := s.QueryReferences(p)
		if err != nil {
			return nil, err
		}
		for _, ref := range refs {
			if !kept[ref] {
				stack = append(stack, ref)
			}
		}
	}
	all, err := s.listValidPaths()
	if err != nil {
		return nil, err
	}
	for _, p := range all {
		if kept[p] {
			continue
		}
		if err := s.deleteValidPath(p); err != nil {
			return nil, err
		}
		deleted = append(deleted, p)
	}
	sort.Strings(deleted)
	return deleted, nil
}

// LockForBuild holds gc.lock shared until the returned function is called.
// Builds take it so that garbage collection waits for them to finish.
func (s *Store) LockForBuild() (unlock func(), err error) {
	return s.lockGC(false)
}

func (s *Store) findGCRoots() (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for _, dir := range []string{s.PhysicalGCRootsDir(), filepath.Join(s.Root, "profiles")} {
		if err := s.walkGCRoot(dir, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Store) walkGCRoot(path string, out map[string]struct{}) error {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := s.walkGCRoot(filepath.Join(path, e.Name()), out); err != nil {
				return err
			}
		}
		return nil
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := store.ReadLinkTarget(path)
		if err != nil {
			return err
		}
		if s.inStore(target) {
			if ok, err := s.IsValidPath(target); err != nil {
				return err
			} else if ok {
				out[target] = struct{}{}
			}
			return nil
		}
		abs := target
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(filepath.Dir(path), target)
		}
		inner, err := os.Lstat(abs)
		if err != nil {
			if os.IsNotExist(err) && s.isAutoRoot(path) {
				_ = os.Remove(path)
			}
			return nil
		}
		if inner.Mode()&os.ModeSymlink == 0 {
			return nil
		}
		storePath, err := store.ReadLinkTarget(abs)
		if err != nil || !s.inStore(storePath) {
			return nil
		}
		if ok, err := s.IsValidPath(storePath); err != nil {
			return err
		} else if ok {
			out[storePath] = struct{}{}
		}
		return nil
	}
	logical := s.StoreDir() + "/" + filepath.Base(path)
	if ok, err := s.IsValidPath(logical); err != nil {
		return err
	} else if ok {
		out[logical] = struct{}{}
	}
	return nil
}

func (s *Store) isAutoRoot(path string) bool {
	auto := filepath.Join(s.PhysicalGCRootsDir(), "auto")
	rel, err := filepath.Rel(auto, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func (s *Store) inStore(path string) bool {
	prefix := s.StoreDir() + "/"
	return strings.HasPrefix(path, prefix) && len(path) > len(prefix)
}

func (s *Store) deleteValidPath(logical string) error {
	phys := PhysicalPath(s.Root, logical)
	if err := RemoveStoreTree(phys); err != nil {
		return err
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM ValidPaths WHERE path = ?`, []any{logical}},
		{`DELETE FROM Refs WHERE referrer = ? OR reference = ?`, []any{logical, logical}},
		{`DELETE FROM DerivationOutputs WHERE path = ? OR drvPath = ?`, []any{logical, logical}},
		{`DELETE FROM GcRoots WHERE path = ?`, []any{logical}},
	} {
		if _, err := tx.Exec(q.sql, q.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func replaceSymlink(link, target string) error {
	tmp := fmt.Sprintf("%s.tmp-%d", link, os.Getpid())
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	return os.Rename(tmp, link)
}

func nix32SHA1(s string) (string, error) {
	sum := sha1.Sum([]byte(s))
	h, err := nixhash.NewHash(nixhash.SHA1, sum[:])
	if err != nil {
		return "", err
	}
	return h.Format(nixhash.NixBase32, false), nil
}
