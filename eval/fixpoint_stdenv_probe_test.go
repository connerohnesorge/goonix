package eval_test

import (
	"path/filepath"
	"testing"
)

func TestFixpointStdenvVsGccStdenvProbeMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "fixpoint_stdenv_vs_gccstdenv_probe.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("fixpoint stdenv/gccStdenv probe differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}

func TestStdenvFolderMergePreservesOverrides(t *testing.T) {
	src := `
let
  stdenv =
    builtins.derivation {
      name = "s";
      system = "x86_64-linux";
      builder = "b";
      shell = "/bin/sh";
    }
    // {
      overrides = self: super: { boot = true; };
      cc = { isGNU = true; };
      hasCC = true;
    };
  args = { stdenv = stdenv; };
  merged = args // { stdenv = args.stdenv // { __bootPackages = { }; }; };
in {
  hasOnArgs = args.stdenv ? overrides;
  hasOnMerged = merged.stdenv ? overrides;
  ccStillSet = merged.stdenv.cc.isGNU or false;
  ccNotNull = merged.stdenv.cc != null;
}
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("folder merge overrides:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
