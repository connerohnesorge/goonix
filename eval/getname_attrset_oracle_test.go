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

func TestGetNameEmptyAttrsetMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `with import <nixpkgs> { config = {}; overlays = []; }; lib.getName { pname = "gcc"; name = "gcc-13.3.0"; }`
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
		t.Fatalf("getName go=%q nix=%q", got, want)
	}
}

func TestGetNameEmptyPnameUsesNameMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `with import <nixpkgs> { config = {}; overlays = []; };
let cc = { pname = ""; name = "gcc-13.3.0"; };
in lib.getName cc`
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
		t.Fatalf("getName empty pname go=%q nix=%q", got, want)
	}
}

func TestGetNameOnlyNameMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `with import <nixpkgs> { config = {}; overlays = []; }; lib.getName { name = "gcc-13.3.0"; }`
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
		t.Fatalf("getName go=%q nix=%q", got, want)
	}
}
