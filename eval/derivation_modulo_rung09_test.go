package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestRung09OutputPathMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	attrs := `let dep = builtins.derivationStrict { name = "rung09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "rung09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`
	wantOut := lay.EvalStrictOutPath(t, attrs)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ae, err := parser.ParseString(attrs)
	if err != nil {
		t.Fatal(err)
	}
	av, err := ev.EvalWithBuiltins(ae)
	if err != nil {
		t.Fatal(err)
	}
	av2, err := ev.ForceValue(av)
	if err != nil {
		t.Fatal(err)
	}
	drv, err := ev.AttrsToDerivation(av2)
	if err != nil {
		t.Fatal(err)
	}
	gotOut := drv.OutPaths["out"]
	if gotOut != wantOut {
		t.Fatalf("out path\nnix=%q\ngo=%q\ninputs=%v", wantOut, gotOut, drv.InputDerivations)
	}
}
