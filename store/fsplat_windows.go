// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package store

import (
	"io/fs"
	"os"
	"path/filepath"
)

// CaseInsensitiveStore reports whether store entry names fold case. NTFS
// directories fold case, so the Windows store uses the case hack.
const CaseInsensitiveStore = true

// ExecStreamName is the NTFS alternate data stream whose presence marks a
// file executable, as NovaNix writes it. NTFS has no execute bit, and the
// NAR needs one for every regular file.
const ExecStreamName = "nova.exec"

func execStreamPath(path string) string { return path + ":" + ExecStreamName }

// IsExecutable reports whether the regular file at path carries the
// executable stream. The stream's content is not read: presence is the bit.
func IsExecutable(path string, fi fs.FileInfo) bool {
	f, err := os.Open(execStreamPath(path))
	if err != nil {
		return false
	}
	f.Close()
	return true
}

// MarkExecutable adds the executable stream to the regular file at path. A
// read-only file cannot gain a stream, so the attribute is cleared for the
// write and put back afterwards.
func MarkExecutable(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	readOnly := fi.Mode().Perm()&0o200 == 0
	if readOnly {
		if err := os.Chmod(path, 0o644); err != nil {
			return err
		}
	}
	werr := os.WriteFile(execStreamPath(path), []byte("1"), 0o644)
	if readOnly {
		if err := os.Chmod(path, 0o444); err != nil && werr == nil {
			werr = err
		}
	}
	return werr
}

// ReadLinkTarget returns the target of the symlink at path with forward
// slashes. Windows stores a link target with backslashes; the NAR spelling,
// and the one the link was created from, uses '/'.
func ReadLinkTarget(path string) (string, error) {
	t, err := os.Readlink(path)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(t), nil
}

// narEntryName is the NAR name of a directory entry called name on disk: the
// case-hack suffix a restore added is dropped.
func narEntryName(name string) string { return stripCaseHack(name) }
