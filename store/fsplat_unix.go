// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package store

import (
	"io/fs"
	"os"
)

// CaseInsensitiveStore reports whether store entry names fold case, so the
// NAR layer applies the case hack.
const CaseInsensitiveStore = false

// IsExecutable reports whether the regular file at path is executable in the
// NAR sense: its owner-execute bit is set, as Nix's dump reads it.
func IsExecutable(path string, fi fs.FileInfo) bool {
	return fi.Mode()&0o100 != 0
}

// MarkExecutable makes the regular file at path executable.
func MarkExecutable(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return os.Chmod(path, fi.Mode().Perm()|0o111)
}

// ReadLinkTarget returns the target of the symlink at path as a NAR stores it.
func ReadLinkTarget(path string) (string, error) {
	return os.Readlink(path)
}

// narEntryName is the NAR name of a directory entry called name on disk.
func narEntryName(name string) string { return name }
