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

// String builtins must preserve !<drvPath>! / store context (Nix getContext).
func TestStringContextAuditMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	pkgs := `import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }`
	cases := []struct {
		name string
		expr string
	}{
		{
			name: "concat",
			expr: `let pkgs = ` + pkgs + `; d = pkgs.gcc-unwrapped; ctx = builtins.getContext (d.outPath + "-suffix"); in builtins.toString (builtins.length (builtins.attrNames ctx))`,
		},
		{
			name: "replaceStrings",
			expr: `let pkgs = ` + pkgs + `; s = pkgs.gcc-unwrapped.outPath; in builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.replaceStrings ["gcc"] ["TOOL"] s))))`,
		},
		{
			name: "substring",
			expr: `let pkgs = ` + pkgs + `; s = pkgs.gcc-unwrapped.outPath; in builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.substring 0 10 s))))`,
		},
		{
			name: "toString",
			expr: `let pkgs = ` + pkgs + `; s = pkgs.gcc-unwrapped.outPath; in builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.toString s))))`,
		},
		{
			name: "unsafeDiscardStringContext",
			expr: `let pkgs = ` + pkgs + `; s = pkgs.gcc-unwrapped.outPath; in builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.unsafeDiscardStringContext s))))`,
		},
		{
			name: "placeholder",
			expr: `builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.placeholder "out"))))`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			want := lay.EvalStdinRaw(t, tc.expr)
			ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
			ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
			ev.NixPath.PrependEntries("nix=" + lay.Nix)
			expr, err := parser.ParseString(tc.expr)
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
				t.Fatalf("go=%q nix=%q", got, want)
			}
		})
	}
}
