// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestParseDerivationShowJSONNix224Flat(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "show_224_flat.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := drvshow.ParseDerivationShowJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := doc.Derivations["fwh0qbryiqv54xj9mrhdky1rj8d9xn8n-xgcc-13.3.0.drv"]
	if !ok {
		t.Fatalf("keys=%v", doc.Derivations)
	}
	if e.Name != "xgcc-13.3.0" {
		t.Fatalf("name=%q", e.Name)
	}
	if len(e.Inputs.Drvs) != 1 {
		t.Fatalf("input drvs=%d", len(e.Inputs.Drvs))
	}
	var found bool
	for k, v := range e.Inputs.Drvs {
		if k != "deadbeef-bootstrap-stage1-stdenv-linux.drv" {
			t.Fatalf("key=%q", k)
		}
		if len(v.Outputs) != 1 || v.Outputs[0] != "out" {
			t.Fatalf("outputs=%v", v.Outputs)
		}
		found = true
	}
	if !found {
		t.Fatal("missing input drv edge")
	}
	if len(e.Inputs.Srcs) != 1 || e.Inputs.Srcs[0] != "/nix/store/cafebabe-glibc-2.40" {
		t.Fatalf("srcs=%v", e.Inputs.Srcs)
	}
}

func TestParseDerivationShowJSONVersion4(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "show_v4.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := drvshow.ParseDerivationShowJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	e := doc.Derivations["250v5qlqzw6x3hdgp708qiwds3m50mxc-t.drv"]
	if e.Name != "t" {
		t.Fatalf("name=%q", e.Name)
	}
}
