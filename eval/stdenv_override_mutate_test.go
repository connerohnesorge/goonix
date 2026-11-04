package eval

import (
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestAttrUpdateOverrideDoesNotMutateOriginal(t *testing.T) {
	src := `
let
  stdenv = { cc = { isGNU = true; }; hasCC = true; };
  stdenvNoCC = stdenv // { cc = null; hasCC = false; };
in stdenv.cc.isGNU
`
	ev := NewEvaluator()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.forceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != VKBool || !fv.Bool {
		t.Fatalf("expected true, got %v", fv)
	}
}
