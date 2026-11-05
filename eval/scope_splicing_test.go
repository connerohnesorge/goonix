package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
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

func TestMakeScopeWithSplicingCallPackageWithAutoMergeMatchesNix(t *testing.T) {
	root := modRoot(t)
	libNix := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  pkg = { lib }: self: with self; { probe = lib.isFunction callPackage; };
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
	lay := oracle.MustLayout(t)
	want := lay.EvalRaw(t, src)
	if want != "true" {
		t.Fatalf("nix probe = %q", want)
	}
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
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("probe = %v", fv)
	}
}
