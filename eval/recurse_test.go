package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestRecurseIntoAttrsTypeInPkgs(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := fmt.Sprintf(`
let pkgs = import %q { system = "x86_64-linux"; config = { }; overlays = [ ]; };
in builtins.typeOf pkgs.recurseIntoAttrs
`, filepath.Join(nixpkgs, "default.nix"))
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
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
	if fv.Kind != eval.VKString || fv.String != "lambda" {
		t.Fatalf("recurseIntoAttrs is %v", fv.DisplayString(ev, ev.Intern))
	}
}
