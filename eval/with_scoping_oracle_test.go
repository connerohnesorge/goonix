package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestWithScopingNestedWithInnermostWins(t *testing.T) {
	assertExprMatchesNix(t, `builtins.toString (let a = { x = 1; }; b = { x = 2; }; in with a; with b; x)`)
}

func TestWithScopingLexicalBeforeWith(t *testing.T) {
	assertExprMatchesNix(t, `builtins.toString (with { x = 1; }; let x = 2; in with { x = 3; }; x)`)
}

func TestWithScopingInnerWithOnAttrBinding(t *testing.T) {
	assertExprMatchesNix(t, `builtins.toString ((p: with p; { y = with { y = 5; }; y; }) { y = 9; }).y`)
}

func TestWithScopingInnerWithInFunctionResult(t *testing.T) {
	assertExprMatchesNix(t, `builtins.toString (let f = s: with s; { z = with { z = 1; }; z; }; in (f { z = 7; }).z)`)
}

func assertExprMatchesNix(t *testing.T, expr string) {
	t.Helper()
	lay := oracle.MustLayout(t)
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("with scoping oracle\nexpr=%s\nnix=%q\ngo=%q", expr, want, got)
	}
}
