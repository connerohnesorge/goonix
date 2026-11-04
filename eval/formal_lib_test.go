package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFormalLibInheritId(t *testing.T) {
	src := `
let
  outer = { id = "ok"; };
  applied = ({ x }: x.id) { x = outer; };
in applied
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
	if fv.Kind != eval.VKString || fv.String != "ok" {
		t.Fatalf("applied kind %v val %v", fv.Kind, fv.DisplayString(ev, ev.Intern))
	}
}
