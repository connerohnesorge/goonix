// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func TestOutputPlaceholderMatchesNixOut(t *testing.T) {
	const want = "/1rz4g4znpzjwh1xymhjpm42vipw92pr73vdgl6xs1hycac8kf2n9"
	if got := store.OutputPlaceholder("out"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestOutputPlaceholderStableForSameName(t *testing.T) {
	a := store.OutputPlaceholder("out")
	b := store.OutputPlaceholder("out")
	if a != b {
		t.Fatalf("%q vs %q", a, b)
	}
}

func TestOutputPlaceholderDiffersByOutputName(t *testing.T) {
	if store.OutputPlaceholder("out") == store.OutputPlaceholder("dev") {
		t.Fatal("out and dev placeholders must differ")
	}
}
