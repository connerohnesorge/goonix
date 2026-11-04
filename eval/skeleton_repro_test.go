package eval_test

import (
	"testing"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestSkeletonLetParsed(t *testing.T) {
	src := `
let
  mkSystem = x: x;
  mkSystemFromSkeleton = {
    cpu,
    kernel,
  }@args:
    let
      getCpu = name: name;
      parsed = {
        cpu = getCpu args.cpu;
        vendor = if false then null else parsed;
      };
    in mkSystem parsed;
in mkSystemFromSkeleton { cpu = "x"; kernel = "linux"; }
`
	expr, err := parser.ParseString(src)
	if err != nil { t.Fatal(err) }
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	ev.ForceValue(val)
}
