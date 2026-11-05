let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in (pkgs.stdenvNoCC.mkDerivation {
  pname = "goonix-stdenv-rung";
  version = "0";
  dontUnpack = true;
  buildCommand = "mkdir $out";
}).drvPath
