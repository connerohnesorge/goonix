# Boot path only: does final pkgs.stdenv retain overrides after folder merge?
{ nixpkgsPath ? ../nixpkgs }:
let
  lib = import (nixpkgsPath + "/lib");
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  crossSystem = localSystem;
  config = { contentAddressedByDefault = false; };
  overlays = [ ];
  crossOverlays = [ ];
  nixpkgsFun = newArgs: import (nixpkgsPath + "/pkgs/top-level") ( {
    inherit lib localSystem crossSystem config overlays crossOverlays;
  } // newArgs );
  allPackages = newArgs: import (nixpkgsPath + "/pkgs/top-level/stage.nix") ( {
    inherit lib nixpkgsFun;
  } // newArgs );
  boot = import (nixpkgsPath + "/pkgs/stdenv/booter.nix") { inherit lib allPackages; };
  stages = (import (nixpkgsPath + "/pkgs/stdenv")) {
    inherit lib localSystem crossSystem config overlays crossOverlays;
  };
  pkgs = boot stages;
in
{
  tryPkgs = (builtins.tryEval pkgs).success;
  hasOverrides = pkgs.stdenv ? overrides;
  tryHasOverrides = (builtins.tryEval (pkgs.stdenv ? overrides)).success;
}
