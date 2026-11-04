package eval

import (
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestSelectDoesNotForceAttrValuesWHNF(t *testing.T) {
	src := `
let
  x = { a = builtins.throw "must not force deep"; };
in x.a
`
	ev := NewEvaluator()
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.evalExpr(expr, env)
	if err != nil {
		t.Fatal(err)
	}
	if val == nil || val.Kind != VKDeferred || val.Deferred == nil {
		t.Fatalf("expected deferred attr binding, got %v", val)
	}
}

func TestSelectAliasChainWHNF(t *testing.T) {
	src := `
let a = x; x = { b = 1; }; in a.b
`
	ev := NewEvaluator()
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
	if fv.Kind != VKInt || fv.Int != 1 {
		t.Fatalf("expected 1, got %v", fv.DisplayString(ev, ev.Intern))
	}
}
