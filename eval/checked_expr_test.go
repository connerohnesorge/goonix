package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFoldrInnerThrowIfNotCall(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  lib = import "` + libPath + `";
  inherit (lib) throwIfNot;
  overlays = [ (x: x) ];
in lib.foldr (x: throwIfNot (lib.isFunction x) "not a function" (y: y) x) (r: r) overlays
`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}

func TestFullCheckedExprFromTopLevel(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  pristineLib = import "` + libPath + `";
  lib = if true then pristineLib else pristineLib;
  inherit (lib) throwIfNot;
  overlays = [ ];
  crossOverlays = [ ];
  checked =
    throwIfNot (lib.isList overlays) "The overlays argument to nixpkgs must be a list." lib.foldr
      (x: throwIfNot (lib.isFunction x) "All overlays passed to nixpkgs must be functions.")
      (r: r)
      overlays
      throwIfNot
      (lib.isList crossOverlays)
      "The crossOverlays argument to nixpkgs must be a list."
      lib.foldr
      (x: throwIfNot (lib.isFunction x) "All crossOverlays passed to nixpkgs must be functions.")
      (r: r)
      crossOverlays;
in checked { }
`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
