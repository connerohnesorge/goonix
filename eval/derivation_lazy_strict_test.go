package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// builtins.derivation must not run derivationStrict until drvPath/outPath is forced (Nix derivation.nix).
func TestDerivationLazyStrictUntilDrvPath(t *testing.T) {
	src := `let
  d = builtins.derivation {
    name = "lazy-strict-test";
    system = "x86_64-linux";
    builder = "/bin/sh";
    bomb = builtins.throw "derivationStrict ran too early";
  };
in d.name
`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	if fv.Kind != eval.VKString || fv.String != "lazy-strict-test" {
		t.Fatalf("got %v", fv)
	}
}

func TestDerivationDrvPathForcesStrict(t *testing.T) {
	src := `let
  d = builtins.derivation {
    name = "lazy-strict-drvpath";
    system = "x86_64-linux";
    builder = "/bin/sh";
  };
in d.drvPath
`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	if fv.Kind != eval.VKPath || fv.Path == "" {
		t.Fatalf("expected drvPath, got %v", fv)
	}
}
