package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestDerivationEnvSpaceSeparatedPathsNotInputSrcs(t *testing.T) {
	src := `builtins.derivationStrict {
	  name = "multi-src-test";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  paths = "/nix/store/aaaaaaa-foo /nix/store/bbbbbbb-bar";
	}`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	p, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	d, ok := ev.RegisteredDerivation(p)
	if !ok {
		t.Fatal("not registered")
	}
	if len(d.InputSources) != 0 {
		t.Fatalf("inputSrcs=%v want none for space-separated env path list", d.InputSources)
	}
}
