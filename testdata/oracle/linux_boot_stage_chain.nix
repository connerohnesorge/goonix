# Linux boot oracle: real stageFuns list length from pkgs/stdenv/linux (stage.nix args)
# plus dfold wiring chain (dfold_nine_stage_order.nix) for __bootPackages walk shape.
{ nixpkgsPath ? ../nixpkgs }:
let
  lib = import (nixpkgsPath + "/lib");
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  config = { contentAddressedByDefault = false; };
  stageFuns = import (nixpkgsPath + "/pkgs/stdenv/linux") {
    inherit lib localSystem config;
    crossSystem = localSystem;
    overlays = [ ];
    crossOverlays = [ ];
  };
  linuxStageCount = builtins.length stageFuns;
  dfold = import ./dfold_nine_stage_order.nix { libPath = nixpkgsPath + "/lib"; };
in
  dfold
  // {
    inherit linuxStageCount;
    stageCount = builtins.length dfold.chain;
  }
