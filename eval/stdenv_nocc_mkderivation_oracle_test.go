// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestStdenvNoCCMkDerivationOracleMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(nixpkgs); err != nil {
		t.Skip("testdata/nixpkgs not present")
	}
	probe := filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix")
	cmd := exec.Command("nix-instantiate", "--eval", "--raw", probe)
	cmd.Env = append(os.Environ(), "NIXPATH=nixpkgs="+nixpkgs)
	nixOut, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix oracle: %v", err)
	}
	want := strings.TrimSpace(string(nixOut))

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
