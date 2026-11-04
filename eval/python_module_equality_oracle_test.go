package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestPythonModuleSelfEqualityMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  py = pkgs.python3Packages.python;
in builtins.toJSON { pySelf = py.pythonModule == py; }`
	want := lay.EvalStdinRaw(t, `builtins.toJSON (`+src+`)`)
	expr, err := parser.ParseString(`builtins.toJSON (` + src + `)`)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("python equality oracle\nnix=%s\ngo=%s", want, got)
	}
}
