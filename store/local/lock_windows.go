// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package local

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// lockGC takes a LockFileEx lock on <root>/gc.lock: exclusive for garbage
// collection, shared for builds. It blocks until the lock is free, like the
// flock the Unix store takes.
func (s *Store) lockGC(exclusive bool) (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.Root, "gc.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	var flags uint32
	if exclusive {
		flags = windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	h := windows.Handle(f.Fd())
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(h, flags, 0, 1, 0, ol); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(h, 0, 1, 0, ol)
		f.Close()
	}, nil
}
