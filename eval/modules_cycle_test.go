package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestEvalModulesEmpty(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(nixpkgs); err != nil {
		t.Skip("testdata/nixpkgs not present")
	}

	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	ev := eval.NewEvaluator()
	ev.SetCurFile(libNix)
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)

	src := `
let
  lib = import "` + libNix + `";
in lib.evalModules { modules = [ ]; }
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
