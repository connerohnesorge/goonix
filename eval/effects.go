// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"io/fs"
	"os"
)

// Effects abstracts filesystem and store mutation for pure vs IO evaluation modes.
type Effects interface {
	ReadFile(path string) ([]byte, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
	// RealiseStorePath materialises a store object at storePath with the given producer.
	RealiseStorePath(storePath string, produce func() error) error
	StoreDir() string
}

// PureEffects rejects IO that would mutate the store; read-only filesystem access is allowed.
type PureEffects struct {
	Root string
}

func (PureEffects) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (PureEffects) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (PureEffects) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (PureEffects) RealiseStorePath(storePath string, produce func() error) error {
	if storePathMaterialised(storePath) {
		return nil
	}
	return produce()
}

func (p PureEffects) StoreDir() string {
	if p.Root == "" {
		return "/nix/store"
	}
	return p.Root
}

// IOEffects allows store writes during evaluation (instantiate, toFile, path coercion).
type IOEffects struct {
	Root string
}

func (IOEffects) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (IOEffects) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (IOEffects) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (e IOEffects) RealiseStorePath(storePath string, produce func() error) error {
	if storePathMaterialised(storePath) {
		return nil
	}
	if err := produce(); err != nil {
		return err
	}
	return nil
}

// storePathMaterialised reports whether storePath already holds a non-empty object.
func storePathMaterialised(storePath string) bool {
	info, err := os.Stat(storePath)
	if err != nil {
		return false
	}
	if !info.IsDir() {
		return true
	}
	entries, err := os.ReadDir(storePath)
	return err == nil && len(entries) > 0
}

func (e IOEffects) StoreDir() string {
	if e.Root == "" {
		return "/nix/store"
	}
	return e.Root
}
