package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestMakeScopeLetSelfMergeWithSelfCallPackageStub(t *testing.T) {
	root := modRoot(t)
	libNix := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  generatedPackages = { lib }: self: with self; { probe = callPackage; };
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
    f = lib.extends (_: _: { }) (lib.callPackageWith { inherit (lib) lib; } generatedPackages { });
  };
in scope.probe
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
