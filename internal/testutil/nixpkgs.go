// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ModuleRoot finds the repository root containing go.mod.
func ModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
}

// NixpkgsPath returns testdata/nixpkgs or skips when the pin is not present.
func NixpkgsPath(t *testing.T) string {
	t.Helper()
	root := ModuleRoot(t)
	p := filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(filepath.Join(p, "default.nix")); err != nil {
		t.Skip("testdata/nixpkgs not present (fetch pinned nixpkgs for oracle tests)")
	}
	return p
}

// NixLibPath returns testdata/nixpkgs/lib/default.nix or skips.
func NixLibPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join(NixpkgsPath(t), "lib", "default.nix")
	if _, err := os.Stat(p); err != nil {
		t.Skip("testdata/nixpkgs/lib not present")
	}
	return p
}
