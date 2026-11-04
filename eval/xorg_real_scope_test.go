package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// Reproduces all-packages xorg: callPackageWith on real default.nix + extends + makeScopeWithSplicing'.
func TestRealXorgMakeScopeCallPackageLookup(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	defNix := filepath.Join(nixpkgs, "pkgs", "servers", "x11", "xorg", "default.nix")

	src := fmt.Sprintf(`
let
  lib = import %q;
  auto = { inherit (lib) lib; pixman = null; fetchurl = lib.trivial.id; };
  generatedPackages = lib.callPackageWith auto (import %q) { };
  scope = lib.makeScopeWithSplicing' {
    splicePackages = x: x;
    newScope = lib.callPackageWith auto;
  } {
    otherSplices = {
      selfBuildBuild = { };
      selfBuildHost = { };
      selfBuildTarget = { };
      selfHostHost = { };
      selfHostTarget = { };
      selfTargetTarget = { };
    };
    f = lib.extends (_: _: { }) generatedPackages;
  };
in scope ? callPackage
`, libNix, defNix)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("expected callPackage on scope, got %s", fv.DisplayString(ev, ev.Intern))
	}
}
