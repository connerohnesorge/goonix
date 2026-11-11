package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestTopLevelLibIfInheritThrowIfNot(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  pristineLib = import "` + libPath + `";
  lib = if true then pristineLib else pristineLib;
  inherit (lib) throwIfNot;
  overlays = [ ];
in throwIfNot (lib.isList overlays) "The overlays argument to nixpkgs must be a list." (x: x) 1
`
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
	if fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestTopLevelCheckedThrowIfNotShape(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  lib = import "` + libPath + `";
  inherit (lib) throwIfNot;
  overlays = [ ];
in throwIfNot (lib.isList overlays) "The overlays argument to nixpkgs must be a list." (x: x) 1
`
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
	if fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
