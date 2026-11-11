// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package testutil

import (
	"os"
	"testing"
)

const chrootEnv = "GOONIX_CHROOT_PHYS"

// SetupChrootStoreEnv creates a temp physical store root and sets GOONIX_CHROOT_PHYS.
func SetupChrootStoreEnv(t *testing.T) func() {
	t.Helper()
	if prev := os.Getenv(chrootEnv); prev != "" {
		// Nested setup (unlikely): keep existing root.
		return func() {}
	}
	root := t.TempDir()
	if err := os.Setenv(chrootEnv, root); err != nil {
		t.Fatal(err)
	}
	return func() {
		_ = os.Unsetenv(chrootEnv)
	}
}

// ChrootStoreRoot returns the physical root for the test chroot store, creating one if needed.
func ChrootStoreRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv(chrootEnv); r != "" {
		return r
	}
	root := t.TempDir()
	if err := os.Setenv(chrootEnv, root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(chrootEnv) })
	return root
}
