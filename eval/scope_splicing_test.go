package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestMakeScopeWithSplicingCallPackageVisible(t *testing.T) {
	root := modRoot(t)
	libNix := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  pkg = { lib }: self: with self; { probe = callPackage; };
  step = lib.makeScopeWithSplicing' {
    splicePackages = x: x;
    newScope = s: x: x // s;
  } {
    otherSplices = {
      selfBuildBuild = { };
      selfBuildHost = { };
      selfBuildTarget = { };
      selfHostHost = { };
      selfHostTarget = { };
      selfTargetTarget = { };
    };
    f = lib.extends (_: _: { }) (lib.callPackageWith { lib = lib; } pkg { });
  };
in step.probe
`, libNix)
	ev := eval.NewEvaluator()
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
	if fv == nil {
		t.Fatal("nil probe")
	}
}

func TestMakeScopeWithSplicingRealXorgDefault(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	defNix := filepath.Join(nixpkgs, "pkgs", "servers", "x11", "xorg", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  auto = { inherit (lib) lib; pixman = null; callPackage = lib.trivial.id; };
  generatedPackages = lib.callPackageWith auto (import %q) { };
  scope = lib.makeScopeWithSplicing' {
    splicePackages = x: x;
    newScope = s: x: x // s;
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
in scope.fontadobeutopiatype1 or scope.appres or "ok"
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
