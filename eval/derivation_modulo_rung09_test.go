package eval_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestRung09OutputPathMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	attrs := `let dep = builtins.derivationStrict { name = "rung09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "rung09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`
	strictArg := "(" + attrs + ")"
	pathExpr := `(builtins.derivationStrict ` + strictArg + `).out`
	nixOut, err := exec.Command("nix-instantiate", "--eval", "--raw", "-E", pathExpr).Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	wantOut := strings.TrimSpace(string(nixOut))

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
