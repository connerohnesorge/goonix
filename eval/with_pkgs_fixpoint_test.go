package eval_test

import "testing"

func TestWithPkgsFixpointProbeMatchesNix(t *testing.T) {
	src := `
let
  lib = import "` + modRoot(t) + `/testdata/nixpkgs/lib";
  bootStdenv = {
    cc = { isGNU = true; };
    hasCC = true;
    overrides = self: super: { bootGlibc = "x"; };
  };
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { stdenv = bootStdenv; })
    (self: super:
      let pkgs = self;
      in with pkgs; {
        gccStdenv = if stdenv.cc.isGNU then stdenv else null;
      })
    (self: super: (super.stdenv.overrides or (_: _: { })) self super)
  ];
  pkgs = lib.fix toFix;
in {
  tryPkgs = (builtins.tryEval pkgs).success;
  tryStdenv = (builtins.tryEval pkgs.stdenv).success;
}
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("with pkgs fixpoint:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
