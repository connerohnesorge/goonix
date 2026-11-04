{ }:
let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  p1 = pkgs.stdenv.drvPath;
  p2 = pkgs.stdenv.drvPath;
in {
  sameInFix = p1 == p2;
  pathNonEmpty = p1 != "";
}
