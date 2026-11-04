package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// Shape of pkgs/stdenv/cross: lib.init bootStages ++ [ stageFn ] with bootStages a list of stages.
func TestLibInitBootStagesCrossShape(t *testing.T) {
	src := `
let
  length = list: builtins.length list;
  take = n: list:
    if n <= 0 then [ ] else
    if n >= length list then list else
    builtins.genList (i: builtins.elemAt list i) n;
  init = list:
    assert length list > 0;
    take (length list - 1) list;
  last = list: builtins.elemAt list (length list - 1);
  lib = { inherit init last; };
  bootStages = [
    { stage = 0; }
    { stage = 1; }
    (somePrev: somePrev // { stage = 2; })
  ];
  extended = lib.init bootStages ++ [
    (somePrev: lib.last bootStages somePrev // { allowCustomOverrides = true; })
  ];
in length extended
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
	if fv.Kind != eval.VKInt || fv.Int != 3 {
		t.Fatalf("expected 3 stages, got %v", fv.DisplayString(ev, ev.Intern))
	}
}
