package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestInheritSelectCallWHNF(t *testing.T) {
	src := `
let
  lib = { throwIfNot = cond: msg: if cond then x: x else throw msg; };
  inherit (lib) throwIfNot;
in throwIfNot true "fail" (x: x) 42
`
	ev := eval.NewEvaluator()
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
	if fv.Kind != eval.VKInt || fv.Int != 42 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestRealLibThrowIfNotCall(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  lib = import "` + libPath + `";
  inherit (lib) throwIfNot;
in throwIfNot true "fail" (x: x) 42
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
	if fv.Kind != eval.VKInt || fv.Int != 42 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
