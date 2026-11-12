// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package local

import "fmt"

// checkEntryName accepts every name a NAR reader accepts; the reader already
// refuses '/', NUL, ".", "..", and the empty name.
func checkEntryName(string) error { return nil }

func symlinkError(l pendingLink, err error) error {
	return fmt.Errorf("creating symlink '%s' -> '%s': %w", l.path, l.target, err)
}
