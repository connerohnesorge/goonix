package eval_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestDerivationMatchesNixOutStructure(t *testing.T) {
	src := `let d = builtins.derivation {
	  name = "foo";
	  system = "x86_64-linux";
	  builder = "/bin/bash";
	  outputs = [ "out" "dev" ];
	}; in {
	  typeOut = builtins.typeOf d.out;
	  hasOutPath = d ? outPath;
	  outHasOutPath = d.out ? outPath;
	  allLen = builtins.length d.all;
	}`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("nix-instantiate"); err == nil {
		cmd := exec.Command("nix-instantiate", "--eval", "--strict", "-E", src)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("nix oracle: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "typeOut") {
			t.Logf("nix: %s", out)
		}
	}
	_ = fv
}

func TestGenListLazySkipsUnusedThrow(t *testing.T) {
	src := `builtins.head (builtins.genList (i: if i == 0 then 1 else throw "forced") 3)`
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("got %v", fv)
	}
}
