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

func oracleEvalString(t *testing.T, lay oracle.Layout, src string) string {
	t.Helper()
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
		t.Fatalf("go=%q nix=%q\nexpr=%s", got, want, src)
	}
	return got
}

func TestParseDrvNameOracleVariants(t *testing.T) {
	lay := oracle.MustLayout(t)
	for _, tc := range []struct{ name, expr string }{
		{"gcc-13.3.0", `(builtins.parseDrvName "gcc-13.3.0").name + "|" + (builtins.parseDrvName "gcc-13.3.0").version`},
		{"gcc-unwrapped", `(builtins.parseDrvName "gcc-unwrapped-13.3.0").name`},
		{"xgcc", `(builtins.parseDrvName "xgcc-13.3.0").name`},
		{"no-version", `(builtins.parseDrvName "gcc").name`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oracleEvalString(t, lay, tc.expr)
		})
	}
}

func TestRemovePrefixEmptyOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; in pkgs.lib.removePrefix "" "gcc"`
	oracleEvalString(t, lay, src)
}

func TestSubstringEdgeCasesOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	cases := []string{
		`builtins.substring 0 3 "gcc"`,
		`builtins.substring 0 (-1) "gcc"`,
		`builtins.substring 0 99 "gcc"`,
		`builtins.substring 1 (-1) "gcc"`,
		`let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; s = pkgs.gcc-unwrapped.outPath; in builtins.substring 0 10 s`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			oracleEvalString(t, lay, c)
		})
	}
}

func TestStringLengthWithContextOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; in builtins.toString (builtins.stringLength pkgs.gcc-unwrapped.outPath)`
	oracleEvalString(t, lay, src)
}

func TestGetNamePnameOrOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	cases := []string{
		`let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; in pkgs.lib.getName { name = "gcc-13.3.0"; }`,
		`let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; in pkgs.lib.getName { pname = "gcc"; name = "gcc-13.3.0"; }`,
		`let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; in pkgs.lib.getName { pname = ""; name = "gcc-13.3.0"; }`,
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			oracleEvalString(t, lay, c)
		})
	}
}
