// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

// RegisterValidPath marks logicalPath valid with optional NAR metadata and refs.
func (s *Store) RegisterValidPath(logicalPath, narHash string, narSize int64, refs []string) error {
	return s.registerPath(logicalPath, narHash, narSize, refs)
}
