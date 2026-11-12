// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !unix && !windows

package local

// lockGC is a no-op where flock is unavailable.
func (s *Store) lockGC(bool) (func(), error) { return func() {}, nil }
