package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFixPrimeLazyLet(t *testing.T) {
	src := `
let
  fix' = f: let x = f x // { tag = "ok"; }; in x;
  f = self: { n = if self ? tag then 1 else 0; };
in (fix' f).tag
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	if val.Kind != eval.VKString || val.String != "ok" {
		t.Fatalf("got %v", val.DisplayString(ev, ev.Intern))
	}
}
