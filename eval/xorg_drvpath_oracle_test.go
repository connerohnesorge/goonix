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
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	nixpkgs := testutil.NixpkgsPath(t)
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + testutil.ModuleRoot(t) + "/testdata/nix")
	parsed, err := parser.ParseString(expr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	switch fv.Kind {
	case eval.VKString:
		return fv.String
	case eval.VKPath:
		return fv.Path
	default:
		t.Fatalf("want string/path, got %v", fv.Kind)
		return ""
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
