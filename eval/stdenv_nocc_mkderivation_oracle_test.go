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

func TestStdenvNoCCMkDerivationOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix")
	want := lay.EvalFileRaw(t, probe)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(probe)
	expr, err := parser.ParseFile(probe)
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
	got := forceStringOrPath(t, ev, fv)
	if got != want {
		t.Fatalf("stdenvNoCC.mkDerivation drvPath\nnix=%q\ngo=%q", want, got)
	}
}

func forceStringOrPath(t *testing.T, ev *eval.Evaluator, v *eval.Value) string {
	t.Helper()
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	switch fv.Kind {
	case eval.VKPath:
		return fv.Path
	case eval.VKString:
		return fv.String
	default:
		t.Fatalf("want path or string, got %v", fv.Kind)
		return ""
	}
}
