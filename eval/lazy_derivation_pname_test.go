// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestLazyDerivationPnameBeforeDrvPath(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `
let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; };
in pkgs.gcc-unwrapped.pname
`
	want := lay.EvalStdinRaw(t, src)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := forceStringOrPathEvalTest(t, ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("pname go=%q nix=%q", got, want)
	}
}
