// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestPinnedNixpkgsBashVersion(t *testing.T) {
	lay := oracle.MustLayout(t)
	v := lay.EvalRaw(t, `with import <nixpkgs> { system = "x86_64-linux"; config = {}; overlays = []; }; bash.version`)
	if v != "5.2p37" {
		t.Fatalf("bash.version=%q want 5.2p37 (oracle nixpkgs pin is wrong)", v)
	}
}
