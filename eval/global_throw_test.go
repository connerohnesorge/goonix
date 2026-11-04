package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestGlobalThrowBuiltinVisible(t *testing.T) {
	expr, err := parser.ParseString(`builtins.throw "msg"`)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	_, err = ev.EvalWithBuiltins(expr)
	if err == nil {
		t.Fatal("expected throw")
	}
}

func TestGlobalThrowNameVisible(t *testing.T) {
	expr, err := parser.ParseString(`throw "msg"`)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	_, err = ev.EvalWithBuiltins(expr)
	if err == nil {
		t.Fatal("expected throw")
	}
}
