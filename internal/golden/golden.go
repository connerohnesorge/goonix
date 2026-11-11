// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package golden supports byte-for-byte comparison against committed oracle files.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

var update = flag.Bool("update", false, "regenerate golden files")

// CompareFile checks got against testdata/golden/<name> under the module root.
func CompareFile(t *testing.T, name string, got []byte) {
	t.Helper()
	goldenPath := filepath.Join(moduleRoot(t), "testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated golden %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update)", goldenPath, err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch for %s\nwant %q\ngot  %q", name, string(want), string(got))
	}
}

// ReadGolden loads a committed golden file from the module tree.
func ReadGolden(t *testing.T, name string) ([]byte, error) {
	t.Helper()
	return os.ReadFile(filepath.Join(moduleRoot(t), "testdata", "golden", name))
}
