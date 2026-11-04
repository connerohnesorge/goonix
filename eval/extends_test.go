package eval_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestExtendsIsCallable(t *testing.T) {
	root, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	src := fmt.Sprintf(`let lib = import %q; g = lib.extends (a: a // { x = 1; }) (a: a // { y = 2; }); in builtins.typeOf g`, libNix)
	expr, _ := parser.ParseString(src)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs="+nixpkgs)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, _ := ev.ForceValue(val)
	if fv.Kind != eval.VKString || fv.String != "lambda" {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
