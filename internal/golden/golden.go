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

var update = flag.Bool("update", false, "regenerate golden files")

// CompareFile checks got against testdata/golden/<name>.
func CompareFile(t *testing.T, name string, got []byte) {
	t.Helper()
	goldenPath := filepath.Join("testdata", "golden", name)
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

// ReadGolden loads a committed golden file.
func ReadGolden(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("testdata", "golden", name))
}
