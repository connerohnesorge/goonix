# Real linux stageFuns + minimal allPackages merge (no all-packages.nix, no asserts skipped).
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

  result = boot stageFuns;

  walk =
    acc: p:
    if p == null then acc
    else if !(p ? stdenv) then
      acc ++ [ { ccType = "no-stdenv"; stdenvName = null; prevStdenvName = null; } ]
    else
      walk (
        acc
        ++ [
          {
            ccType = builtins.typeOf p.stdenv.cc;
            stdenvName = p.stdenv.name or null;
            prevStdenvName =
              if (p.stdenv.__bootPackages or null) != null && (p.stdenv.__bootPackages ? stdenv) then
                p.stdenv.__bootPackages.stdenv.name or null
              else
                null;
          }
        ]
      ) (p.stdenv.__bootPackages or null);

  chain = walk [ ] result;
  chainByDfoldIndex = builtins.genList (i: builtins.elemAt chain ((builtins.length chain) - 1 - i)) (builtins.length chain);
  stageCount = builtins.length chain;
in
{
  inherit chain chainByDfoldIndex stageCount;
  finalCcType = builtins.typeOf result.stdenv.cc;
  finalStdenvName = result.stdenv.name or null;
}
