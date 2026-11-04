# Evaluates the same boot path as pkgs/default.nix and reports stdenv.cc type
# without forcing all-packages attributes like gccStdenv.
let
  lib = import ../nixpkgs/lib;
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  crossSystem = localSystem;
  config = { };
  overlays = [ ];
  crossOverlays = [ ];
  nixpkgsFun = newArgs: import ../nixpkgs/pkgs/top-level ( {
    inherit lib localSystem crossSystem config overlays crossOverlays;
  } // newArgs );
  allPackages = newArgs: import ../nixpkgs/pkgs/top-level/stage.nix ( {
    inherit lib nixpkgsFun;
  } // newArgs );
  boot = import ../nixpkgs/pkgs/stdenv/booter.nix { inherit lib allPackages; };
  stages = (import ../nixpkgs/pkgs/stdenv) {
    inherit lib localSystem crossSystem config overlays crossOverlays;
  };
  pkgs = boot stages;
in
{
  ccType = builtins.typeOf pkgs.stdenv.cc;
  hasCC = pkgs.stdenv.hasCC or null;
}
