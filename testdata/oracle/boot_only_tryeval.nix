# Boot-only tryEval (stub allPackages).
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

  allPackages = args:
    let
      pred = args.stdenv.__bootPackages or { };
    in
    pred
    // args
    // {
      stdenv = args.stdenv;
      pkgs = pred // args;
      buildPackages = pred // args;
    };

  boot = import (nixpkgsPath + "/pkgs/stdenv/booter.nix") { inherit lib allPackages; };
  pkgs = boot stageFuns;
in
(builtins.tryEval pkgs).success
