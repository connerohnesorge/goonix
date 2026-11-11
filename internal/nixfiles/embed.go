// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package nixfiles

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:data
var files embed.FS

// MaterializedDir extracts embedded <nix/…> support files to a temp directory.
func MaterializedDir() (string, error) {
	root, err := fs.Sub(files, "data")
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp("", "goonix-embedded-nix-")
	if err != nil {
		return "", err
	}
	err = fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
