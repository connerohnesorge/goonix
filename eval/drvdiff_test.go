// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/drvdiff"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func evalProbeDrvPath(t *testing.T, probe string) (string, *eval.Evaluator) {
	t.Helper()
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
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
	got, err := forceStringOrPathEvalTest(t, ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	return got, ev
}

func forceStringOrPathEvalTest(t *testing.T, ev *eval.Evaluator, v *eval.Value) (string, error) {
	t.Helper()
	fv, err := ev.ForceValue(v)
	if err != nil {
		return "", err
	}
	switch fv.Kind {
	case eval.VKPath:
		return fv.Path, nil
	case eval.VKString:
		return fv.String, nil
	default:
		t.Fatalf("want path or string, got %v", fv.Kind)
		return "", nil
	}
}

func TestDrvDiffStdenvNoCCProbeReportsDeepest(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	inst := filepath.Join(root, "eval", "testdata", "stdenv_nocc_inst.nix")
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	nixEnv := []string{"NIXPATH=nixpkgs=" + filepath.Join(root, "testdata", "nixpkgs")}
	nixPath, err := drvshow.MaterializeNixDrv(inst, append(os.Environ(), nixEnv...))
	if err != nil {
		t.Fatalf("materialize nix: %v", err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		t.Fatalf("nix closure: %v", err)
	}
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatalf("go closure: %v", err)
	}
	t.Logf("strict derivations=%d modulo cache=%d", ev.StrictDerivationCount(), ev.ModuloCacheCount())
	nixKey := drvshow.DrvKey(nixPath)
	goKey := drvshow.DrvKey(goPath)
	report, err := drvdiff.Analyse(nixDoc, goDoc, nixKey, goKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("closure sizes nix=%d go=%d", len(nixDoc.Derivations), len(goDoc.Derivations))
	t.Log(drvdiff.FormatReport(report))
	if report.Deepest != nil {
		t.Fatalf("deepest field mismatch:\n%s", drvdiff.FormatResult(report.Deepest))
	}
	if len(report.MissingInGo) > 0 {
		t.Fatalf("go closure missing %d derivations (strict=%d); first missing: %q",
			len(report.MissingInGo), ev.StrictDerivationCount(), report.MissingInGo[0])
	}
}

func TestDrvDiffHelloGoldenClosure(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	if testing.Short() {
		t.Skip("hello closure is large")
	}
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	helloNix := filepath.Join(root, "testdata", "ladder", "hello.nix")
	nixEnv := []string{"NIXPATH=nixpkgs=" + nixpkgs + ":nix=" + filepath.Join(root, "testdata", "nix")}
	nixPath, err := drvshow.MaterializeNixDrv(helloNix, append(os.Environ(), nixEnv...))
	if err != nil {
		t.Fatalf("materialize hello: %v", err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	ev.SetCurFile(helloNix)
	expr, err := parser.ParseFile(helloNix)
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
	goPath, err := eval.ForceDerivationAttrs(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		t.Fatal(err)
	}
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatalf("go closure: %v", err)
	}
	res, err := drvdiff.Compare(nixDoc, goDoc, drvshow.DrvKey(nixPath), drvshow.DrvKey(goPath))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(drvdiff.FormatResult(res))
	_ = strings.TrimSpace
}
