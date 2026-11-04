package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestCallPackageWithXorgDefaultSelfCallPackage(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	defNix := filepath.Join(nixpkgs, "pkgs", "servers", "x11", "xorg", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  auto = { inherit (lib) lib; pixman = null; };
  generated = lib.callPackageWith auto (import %q) { };
  self = generated self // { callPackage = 123; };
in self.fontadobeutopiatype1 or self.xorgproto or "ok"
`, libNix, defNix)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
