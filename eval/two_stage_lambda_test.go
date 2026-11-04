package eval_test

import (
	"testing"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestTwoStageLambdaPartialApply(t *testing.T) {
	src := `let f = a: b: { va = a; vb = b; }; in builtins.typeOf (f "A")`
	ev := eval.NewEvaluator()
	expr, _ := parser.ParseString(src)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.ForceValue(val)
	if fv.Kind != eval.VKString || fv.String != "lambda" {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
