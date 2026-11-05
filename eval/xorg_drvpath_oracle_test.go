package eval_test

import (
	"fmt"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/internal/testutil"
	"github.com/connerohnesorge/goonix/parser"
)

func evalPkgsExprString(t *testing.T, expr string) string {
	t.Helper()
	s, err := evalPkgsExprStringAllowError(t, expr)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func evalPkgsExprStringAllowError(t *testing.T, expr string) (string, error) {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{}).WithExperimental("flakes")
	nixpkgs := testutil.NixpkgsPath(t)
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + testutil.ModuleRoot(t) + "/testdata/nix")
	parsed, err := parser.ParseString(expr)
	if err != nil {
		return "", err
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		return "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	switch fv.Kind {
	case eval.VKString:
		return fv.String, nil
	case eval.VKPath:
		return fv.Path, nil
	default:
		return "", fmt.Errorf("want string/path, got %v", fv.Kind)
	}
}

func pkgsImportExpr(nixpkgs string) string {
	return fmt.Sprintf(`import %q {
    localSystem = { system = "x86_64-linux"; };
    config = { contentAddressedByDefault = false; };
    overlays = [ ];
  }`, nixpkgs)
}

func TestXorgXcbprotoDrvPathMatchesNixOracle(t *testing.T) {
	nixpkgs := testutil.NixpkgsPath(t)
	expr := fmt.Sprintf(`let pkgs = %s; in pkgs.xorg.xcbproto.drvPath`, pkgsImportExpr(nixpkgs))
	lay := oracle.MustLayout(t)
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("xcbproto.drvPath\nnix=%q\ngo=%q", want, got)
	}
}

func TestRealXorgMakeScopeCallPackageXcbprotoDrvPathMatchesNix(t *testing.T) {
	nixpkgs := testutil.NixpkgsPath(t)
	libNix := nixpkgs + "/lib/default.nix"
	defNix := nixpkgs + "/pkgs/servers/x11/xorg/default.nix"
	expr := fmt.Sprintf(`
let
  lib = import %q;
  pkgs = %s;
  auto = pkgs // { inherit (lib) lib; pixman = null; fetchurl = lib.trivial.id; };
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
in pkgs.xorg.xcbproto.drvPath
`, libNix, pkgsImportExpr(nixpkgs), defNix)
	lay := oracle.MustLayout(t)
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("real xorg scope wiring xcbproto.drvPath\nnix=%q\ngo=%q", want, got)
	}
}
