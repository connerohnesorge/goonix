// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import "testing"

// MustLayout finds testdata paths and verifies the nixpkgs pin.
func MustLayout(t *testing.T) Layout {
	t.Helper()
	SkipIfUnavailable(t)
	l, err := FindLayout()
	if err != nil {
		t.Fatal(err)
	}
	l.AssertPinned(t)
	return l
}
