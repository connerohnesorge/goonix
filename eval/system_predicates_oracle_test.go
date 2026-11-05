package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestPlainSystemAttrsetIsLinuxThrowsLikeNix(t *testing.T) {
	expr := `{ system = "x86_64-linux"; }.isLinux`
	lay := oracle.MustLayout(t)
	_, err := lay.EvalExprAllowError(t, expr)
	if err == nil {
		t.Fatal("expected nix to reject plain system.isLinux")
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(mustParse(t, expr))
	if err != nil {
		return
	}
	_, err = ev.ForceValue(val)
	if err == nil {
		t.Fatal("expected go to reject plain system.isLinux")
	}
}

func mustParse(t *testing.T, src string) parser.Expr {
	t.Helper()
	e, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	return e
}
