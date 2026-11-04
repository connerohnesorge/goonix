let
  pkgs = import <nixpkgs> { config = { }; overlays = [ ]; };
in
pkgs.stdenvNoCC.mkDerivation {
  pname = "goonix-stdenv-rung";
  version = "0";
  dontUnpack = true;
  buildCommand = "mkdir $out";
}
