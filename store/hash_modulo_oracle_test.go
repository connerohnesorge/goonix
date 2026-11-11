// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func nixDrvPath(t *testing.T, expr string) string {
	t.Helper()
	lay := oracle.MustLayout(t)
	return lay.EvalRaw(t, expr)
}

func goDrvPath(t *testing.T, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	src := "let a = " + strictArg + "; in builtins.derivationStrict a"
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	return mustForceDrvPath(t, ev, fv)
}

func mustForceDrvPath(t *testing.T, ev *eval.Evaluator, v *eval.Value) string {
	t.Helper()
	p, err := eval.ForceStrictDrvPath(ev, v)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Two-level chain: top depends on dep.out (rung 9 shape).
func TestHashModuloOracleTwoLevel(t *testing.T) {
	attrs := `let dep = builtins.derivationStrict { name = "mod09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "mod09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`
	nix := nixDrvPathStrict(t, attrs)
	goPath := goDrvPath(t, attrs)
	if nix != goPath {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", nix, goPath)
	}
}

// Three-level chain with fixed-output middle (FOD leaf, regular top).
func TestHashModuloOracleThreeLevelWithFOD(t *testing.T) {
	attrs := `let
	  leaf = builtins.derivationStrict { name = "mod-chain-leaf"; system = "x86_64-linux"; builder = "/bin/sh"; };
	  fod = builtins.derivationStrict {
	    name = "mod-chain-fod";
	    system = "x86_64-linux";
	    builder = "builtin:fetchurl";
	    outputHashAlgo = "sha256";
	    outputHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
	    outputHashMode = "flat";
	  };
	in {
	  name = "mod-chain-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  x = fod.out;
	  y = leaf.out;
	}`
	nix := nixDrvPathStrict(t, attrs)
	goPath := goDrvPath(t, attrs)
	if nix != goPath {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", nix, goPath)
	}
}

func nixDrvPathStrict(t *testing.T, attrs string) string {
	t.Helper()
	lay := oracle.MustLayout(t)
	return lay.EvalStrictDrvPath(t, attrs)
}

func TestHashModuloOraclePremiseInputUsesUnmaskedDep(t *testing.T) {
	oracle.MustLayout(t)
	// Changing a non-output env var on dep must affect top drvPath (dep hashed unmasked).
	base := `let dep = builtins.derivationStrict { name = "premise-dep"; system = "x86_64-linux"; builder = "/bin/sh"; TAG = "a"; }; in {
	  name = "premise-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`
	alt := strings.Replace(base, `TAG = "a"`, `TAG = "b"`, 1)
	nixBase := nixDrvPathStrict(t, base)
	nixAlt := nixDrvPathStrict(t, alt)
	if nixBase == nixAlt {
		t.Fatalf("oracle premise: expected top hash to depend on dep env (unmasked pathDerivationModulo)")
	}
	goBase := goDrvPath(t, base)
	goAlt := goDrvPath(t, alt)
	if goBase == goAlt {
		t.Fatalf("goonix: top drvPath should depend on dep env under unmasked dep modulo")
	}
}
