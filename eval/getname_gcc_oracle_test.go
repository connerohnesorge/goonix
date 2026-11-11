// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestGetNameGccUnwrappedMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	src := `
let
  pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; };
  inherit (pkgs) lib;
in lib.getName pkgs.gcc-unwrapped + "-wrapper"
`
	want := lay.EvalStdinRaw(t, src)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(filepath.Join(root, "getname_gcc.nix"))
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
		t.Fatalf("getName gcc-unwrapped wrapper prefix go=%q nix=%q", got, want)
	}
}

func TestGetNameBinutilsUnwrappedMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `
let
  pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; };
  inherit (pkgs) lib;
in lib.getName pkgs.binutils-unwrapped + "-wrapper"
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
		t.Fatalf("getName binutils-unwrapped go=%q nix=%q", got, want)
	}
}
