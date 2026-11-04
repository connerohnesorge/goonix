package eval_test

import (
	"path/filepath"
	"testing"
)

func TestBootTake2GlibcPassthruMatchesNix(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	defPath := filepath.Join(root, "testdata", "nixpkgs", "default.nix")
	src := `
let
  lib = import "` + libPath + `";
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  pkgs = import "` + defPath + `" {
    inherit localSystem;
    crossSystem = localSystem;
    config = { };
    overlays = [ ];
    crossOverlays = [ ];
    stdenvStages = args: lib.take 2 (import "` + filepath.Join(root, "testdata", "nixpkgs", "pkgs/stdenv") + `" args);
  };
in {
  tryGlibc = (builtins.tryEval pkgs.glibc).success;
  tryBoot = (builtins.tryEval (pkgs.glibc.passthru.isFromBootstrapFiles or false)).success;
  bootVal = if (builtins.tryEval (pkgs.glibc.passthru.isFromBootstrapFiles or false)).success then (pkgs.glibc.passthru.isFromBootstrapFiles or false) else false;
}
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("k=2 glibc passthru:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
