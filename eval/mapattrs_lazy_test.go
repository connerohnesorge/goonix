package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestMapAttrsLazySkipsUnusedThrow(t *testing.T) {
	src := `
let
  out = builtins.mapAttrs (n: v: v + 1) {
    a = 1;
    b = throw "unused";
  };
in out.a
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
	if fv.Kind != eval.VKInt || fv.Int != 2 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestReadDirReturnsAttrset(t *testing.T) {
	src := `builtins.attrNames (builtins.readDir ".")`
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
	if fv.Kind != eval.VKList {
		t.Fatalf("expected list of names, got %s", fv.DisplayString(ev, ev.Intern))
	}
}
