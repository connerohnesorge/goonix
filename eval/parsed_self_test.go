package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestLambdaLetParsedSelfInPredicate(t *testing.T) {
	src := `
let
  isDarwin = p: false;
  isLinux = p: true;
  mk = args: let
    parsed = {
      vendor = if isDarwin parsed then 1 else if isLinux parsed then 2 else 0;
    };
  in parsed.vendor;
in mk { cpu = "x"; kernel = "linux"; }
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
	if fv.Kind != eval.VKInt || fv.Int != 2 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestLambdaOptionalFormalsLetParsed(t *testing.T) {
	src := `
let
  mkSystem = x: x;
  mk = { cpu, vendor ? assert false; null, kernel }@args: let
    parsed = { ok = true; };
  in mkSystem parsed;
in mk { cpu = "x"; kernel = "linux"; }
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
	ev.ForceValue(val)
}

func TestLambdaFormalsLetBodyCallsWithParsed(t *testing.T) {
	src := `
let
  mkSystem = x: x;
  mk = { cpu, kernel }@args: let
    parsed = { ok = true; };
  in mkSystem parsed;
in mk { cpu = "x"; kernel = "linux"; }
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
	ev.ForceValue(val)
}

func TestLambdaLetBodyCallsWithParsed(t *testing.T) {
	src := `
let
  mkSystem = x: x;
  mk = args: let
    parsed = { ok = true; };
  in mkSystem parsed;
in mk { cpu = "x"; kernel = "linux"; }
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
	ev.ForceValue(val)
}
