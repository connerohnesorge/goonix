package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestWithSelfCallPackageFromMerge(t *testing.T) {
	src := `
let
  f = self: with self; { x = callPackage; };
  self = f self // { callPackage = 42; };
in self.x
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

func TestWithSelfCallPackageTwoStage(t *testing.T) {
	src := `
let
  pkg = { lib }: self: with self; { font = callPackage; };
  step1 = pkg { lib = { }; };
  self = step1 self // { callPackage = 99; };
in self.font
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
	if fv.Kind != eval.VKInt || fv.Int != 99 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
