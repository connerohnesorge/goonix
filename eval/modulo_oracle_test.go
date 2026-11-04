// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

func strictEvalTop(t *testing.T, attrs string) (drvPath string, outPath string, ev *Evaluator) {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	src := "let a = " + strictArg + "; in builtins.derivationStrict a"
	ev = NewEvaluator().WithEffects(IOEffects{})
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
	drvPath, err = ForceDerivationAttrs(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	outSym := ev.Intern.Intern("out")
	b := fv.Attrs.Binds[outSym]
	if b == nil {
		t.Fatal("missing out")
	}
	ov, err := ev.attrsBindingForce(b)
	if err != nil {
		t.Fatal(err)
	}
	outPath, err = ev.forceString(ov)
	if err != nil {
		t.Fatal(err)
	}
	return drvPath, outPath, ev
}

func nixStrictOutPath(t *testing.T, attrs string) string {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := "(builtins.derivationStrict " + strictArg + ").out"
	out, err := exec.Command("nix-instantiate", "--eval", "--raw", "-E", pathExpr).Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestModuloTwoLevelOutPathVsNix(t *testing.T) {
	attrs := `let dep = builtins.derivationStrict { name = "mod09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "mod09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`
	want := nixStrictOutPath(t, attrs)
	drvPath, gotOut, ev := strictEvalTop(t, attrs)
	gd, ok := ev.moduloCache.Lookup(drvPath)
	if !ok {
		t.Fatalf("top %q not in modulo cache", drvPath)
	}
	reps, err := store.InputReplacementsHex(ev.moduloCache, gd)
	if err != nil {
		t.Fatal(err)
	}
	outGoNix, err := gd.CalculateOutputPaths(reps)
	if err != nil {
		t.Fatal(err)
	}
	if outGoNix["out"] != want {
		t.Fatalf("go-nix CalculateOutputPaths\nnix=%q\ngo=%q\nstrict=%q\nreps=%v", want, outGoNix["out"], gotOut, reps)
	}
	outMod, err := store.OutputPathsModulo(store.Config{}, gd, ev.moduloCache)
	if err != nil {
		t.Fatal(err)
	}
	if outMod["out"] != want {
		t.Fatalf("OutputPathsModulo\nnix=%q\ngo=%q", want, outMod["out"])
	}
	if gotOut != want {
		t.Fatalf("strict out\nnix=%q\ngo=%q", want, gotOut)
	}
}

func TestModuloDepReplacementMatchesGoNix(t *testing.T) {
	attrs := `{ name = "mod09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }`
	depPath, _, ev := strictEvalTop(t, attrs)
	gd, ok := ev.moduloCache.Lookup(depPath)
	if !ok {
		t.Fatalf("dep %q not in cache", depPath)
	}
	res, err := ev.moduloCache.PathDerivationModulo(depPath)
	if err != nil {
		t.Fatal(err)
	}
	repl, err := gd.CalculateDrvReplacement(nil)
	if err != nil {
		t.Fatal(err)
	}
	our := hexEncode(res.Hashes["out"])
	if our != repl {
		t.Fatalf("dep replacement\nmodulo=%q\ngonix=%q", our, repl)
	}
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0xf]
	}
	return string(out)
}

func TestModuloDepOutPathVsNix(t *testing.T) {
	attrs := `{ name = "mod09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }`
	want := nixStrictOutPath(t, attrs)
	_, got, _ := strictEvalTop(t, attrs)
	if got != want {
		t.Fatalf("dep out\nnix=%q\ngo=%q", want, got)
	}
}
