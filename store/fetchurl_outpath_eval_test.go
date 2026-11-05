package store_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func evalFetchurlOutPathGo(t *testing.T, expr string) string {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	parsed, err := parser.ParseString(expr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind == eval.VKString {
		return fv.String
	}
	return fv.Path
}
