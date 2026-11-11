// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"io/fs"
	"os"
	"strings"

	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

const chrootPhysEnv = "GOONIX_CHROOT_PHYS"

// Effects abstracts filesystem and store mutation for pure vs IO evaluation modes.
type Effects interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
	// RealiseStorePath materialises a store object at storePath with the given producer.
	RealiseStorePath(storePath string, produce func() error) error
	StoreDir() string
}

func chrootPhysRootFromEnv() string {
	return os.Getenv(chrootPhysEnv)
}

func mapLogicalStorePath(physRoot, path string) string {
	if physRoot == "" || !strings.HasPrefix(path, store.DefaultStoreDir+"/") {
		return path
	}
	return local.PhysicalPath(physRoot, path)
}

func readFileChroot(physRoot, path string) ([]byte, error) {
	mapped := mapLogicalStorePath(physRoot, path)
	b, err := os.ReadFile(mapped)
	if err == nil || physRoot == "" || !strings.HasPrefix(path, store.DefaultStoreDir+"/") {
		return b, err
	}
	if !os.IsNotExist(err) {
		return b, err
	}
	return os.ReadFile(path)
}

func readDirChroot(physRoot, path string) ([]fs.DirEntry, error) {
	return os.ReadDir(mapLogicalStorePath(physRoot, path))
}

func statChroot(physRoot, path string) (fs.FileInfo, error) {
	return os.Stat(mapLogicalStorePath(physRoot, path))
}

func realiseChroot(physRoot, storePath string, produce func() error) error {
	phys := mapLogicalStorePath(physRoot, storePath)
	if storePathMaterialised(phys) {
		return nil
	}
	return produce()
}

// PureEffects rejects IO that would mutate the store; read-only filesystem access is allowed.
type PureEffects struct {
	Root string
}

func (p PureEffects) physRoot() string {
	if p.Root != "" {
		return p.Root
	}
	return chrootPhysRootFromEnv()
}

func (p PureEffects) ReadFile(path string) ([]byte, error) {
	return readFileChroot(p.physRoot(), path)
}

func (p PureEffects) ReadDir(path string) ([]fs.DirEntry, error) {
	return readDirChroot(p.physRoot(), path)
}

func (p PureEffects) Stat(path string) (fs.FileInfo, error) {
	return statChroot(p.physRoot(), path)
}

func (p PureEffects) RealiseStorePath(storePath string, produce func() error) error {
	return realiseChroot(p.physRoot(), storePath, produce)
}

func (p PureEffects) StoreDir() string {
	if p.Root == "" {
		return store.DefaultStoreDir
	}
	return p.Root
}

// IOEffects allows store writes during evaluation (instantiate, toFile, path coercion).
type IOEffects struct {
	Root string
}

func (e IOEffects) physRoot() string {
	return chrootPhysRootFromEnv()
}

func (e IOEffects) ReadFile(path string) ([]byte, error) {
	return readFileChroot(e.physRoot(), path)
}

func (e IOEffects) ReadDir(path string) ([]fs.DirEntry, error) {
	return readDirChroot(e.physRoot(), path)
}

func (e IOEffects) Stat(path string) (fs.FileInfo, error) {
	return statChroot(e.physRoot(), path)
}

func (e IOEffects) RealiseStorePath(storePath string, produce func() error) error {
	return realiseChroot(e.physRoot(), storePath, produce)
}

func (e IOEffects) StoreDir() string {
	if e.Root == "" {
		return store.DefaultStoreDir
	}
	return e.Root
}

// storePathMaterialised reports whether storePath already holds a non-empty object.
func storePathMaterialised(storePath string) bool {
	info, err := os.Stat(storePath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		entries, err := os.ReadDir(storePath)
		return err == nil && len(entries) > 0
	}
	return true
}

// ChrootIOEffects keeps logical paths under /nix/store but materialises objects under
// local.OpenStore's physical layout (PhysRoot/store/<basename>).
type ChrootIOEffects struct {
	PhysRoot string
}

func (e ChrootIOEffects) ReadFile(path string) ([]byte, error) {
	return readFileChroot(e.PhysRoot, path)
}

func (e ChrootIOEffects) ReadDir(path string) ([]fs.DirEntry, error) {
	return readDirChroot(e.PhysRoot, path)
}

func (e ChrootIOEffects) Stat(path string) (fs.FileInfo, error) {
	return statChroot(e.PhysRoot, path)
}

func (ChrootIOEffects) StoreDir() string {
	return store.DefaultStoreDir
}

func (e ChrootIOEffects) physicalStorePath(logical string) string {
	return mapLogicalStorePath(e.PhysRoot, logical)
}

func (e ChrootIOEffects) RealiseStorePath(storePath string, produce func() error) error {
	return realiseChroot(e.PhysRoot, storePath, produce)
}

func defaultEffects() Effects {
	if root := chrootPhysRootFromEnv(); root != "" {
		return ChrootIOEffects{PhysRoot: root}
	}
	return PureEffects{}
}

func coalesceEffects(eff Effects) Effects {
	if eff != nil {
		if root := chrootPhysRootFromEnv(); root != "" {
			switch eff.(type) {
			case IOEffects, PureEffects:
				return ChrootIOEffects{PhysRoot: root}
			}
		}
		return eff
	}
	return defaultEffects()
}

// physicalStorePath maps a logical store path to the filesystem path used for IO.
func (ev *Evaluator) physicalStorePath(logical string) string {
	switch e := ev.Effects.(type) {
	case ChrootIOEffects:
		return e.physicalStorePath(logical)
	default:
		if root := chrootPhysRootFromEnv(); root != "" {
			return mapLogicalStorePath(root, logical)
		}
	}
	return logical
}

// ReadOnlyEffects wraps another Effects and never writes the store: store
// paths are still computed, as with Nix's --readonly-mode. goonix eval uses
// it, since it has no --store and a normal user cannot write /nix/store.
type ReadOnlyEffects struct {
	Effects
}

// RealiseStorePath does nothing; the caller already has the store path.
func (ReadOnlyEffects) RealiseStorePath(string, func() error) error { return nil }
