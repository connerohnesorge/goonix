// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"fmt"
	"os"
	"path/filepath"
)

// Layout is pinned testdata paths under the module root.
type Layout struct {
	Root     string
	Nixpkgs  string
	Nix      string
	Testdata string
}

// FindLayout returns paths relative to the directory containing go.mod.
func FindLayout() (Layout, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Layout{}, err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			td := filepath.Join(dir, "testdata")
			return Layout{
				Root:     dir,
				Testdata: td,
				Nixpkgs:  filepath.Join(td, "nixpkgs"),
				Nix:      filepath.Join(td, "nix"),
			}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return Layout{}, fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}
