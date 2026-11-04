// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestFetchurlSRIOutputHashAlgoEmptyMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `
let
  fetchurl = import <nix/fetchurl.nix>;
in (fetchurl {
  url = "https://ftpmirror.gnu.org/bison/bison-3.8.2.tar.gz";
  hash = "sha256-BsnhO99+sk1M62tZIFpPZ8LH5yExGWREMP6C+9FKCrs=";
}).drvPath
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile("fetchurl_sri_oracle.nix")
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	goPath := forceStringOrPath(t, ev, fv)
	wantPath := lay.EvalStdinRaw(t, src)
	if goPath != wantPath {
		t.Fatalf("drvPath go=%q nix=%q", goPath, wantPath)
	}
	gd, ok := ev.RegisteredDerivation(goPath)
	if !ok {
		t.Fatalf("go drv %q not in strict registry", goPath)
	}
	if gd.Env["outputHashAlgo"] != "" {
		t.Fatalf("outputHashAlgo=%q want empty for SRI", gd.Env["outputHashAlgo"])
	}
	if gd.Env["builder"] != "builtin:fetchurl" {
		t.Fatalf("env builder=%q", gd.Env["builder"])
	}
	if gd.Env["system"] != "builtin" {
		t.Fatalf("system=%q", gd.Env["system"])
	}
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := goDoc.Derivations[drvshow.DrvKey(goPath)]
	if entry.Env["outputHashAlgo"] != "" {
		t.Fatalf("show env outputHashAlgo=%q", entry.Env["outputHashAlgo"])
	}
}
