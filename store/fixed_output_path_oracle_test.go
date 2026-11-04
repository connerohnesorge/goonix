// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/store"
)

func TestMakeFixedOutputPathFetchurlBisonMatchesNixATerm(t *testing.T) {
	// From Nix 2.24.9 hermetic oracle (bison fetchurl flat sha256 SRI).
	want := "/nix/store/9vm1ihdg1ysmrjdbb80g834iizzxb4yk-bison-3.8.2.tar.gz"
	base16, drvAlgo, err := store.ParseFixedOutputHash(
		"sha256-BsnhO99+sk1M62tZIFpPZ8LH5yExGWREMP6C+9FKCrs=",
		"",
		"flat",
	)
	if err != nil {
		t.Fatal(err)
	}
	if drvAlgo != "sha256" {
		t.Fatalf("drvAlgo=%q", drvAlgo)
	}
	d := &gonixdrv.Derivation{
		Outputs: map[string]*gonixdrv.Output{
			"out": {Hash: base16, HashAlgorithm: drvAlgo},
		},
		Env: map[string]string{
			"outputHashMode": "flat",
			"name":           "bison-3.8.2.tar.gz",
		},
	}
	cfg := store.Config{}
	got, err := store.MakeFixedOutputPath(cfg, d, "out", d.Outputs["out"])
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("outPath go=%q want=%q", got, want)
	}
}
