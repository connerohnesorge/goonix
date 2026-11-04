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

func TestGetNameGccUnwrappedOverrideAttrsMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `
let
  pkgs = import <nixpkgs> { config = {}; overlays = []; };
  cc = pkgs.gcc-unwrapped.overrideAttrs (a: { passthru = a.passthru // { bootProbe = true; }; });
in pkgs.lib.getName cc + "-wrapper"
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
		t.Fatalf("getName overrideAttrs go=%q nix=%q", got, want)
	}
}
