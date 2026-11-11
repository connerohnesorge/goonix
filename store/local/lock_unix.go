// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package local

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lockGC takes a flock on <root>/gc.lock: exclusive for garbage collection,
// shared for builds. It blocks until the lock is free.
func (s *Store) lockGC(exclusive bool) (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.Root, "gc.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	if err := unix.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}, nil
}
