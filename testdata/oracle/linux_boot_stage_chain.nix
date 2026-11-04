# Full linux boot stage chain via __bootPackages (same fields as production boot).
{ nixpkgsPath ? ../nixpkgs }:
let
  pkgs = import (nixpkgsPath + "/default.nix") {
    localSystem = { system = "x86_64-linux"; };
    config = { };
    overlays = [ ];
  };

  walk =
    acc: p:
    if p == null || !(p ? stdenv) then acc
    else
      walk (
        acc
        ++ [
          {
            ccType = builtins.typeOf p.stdenv.cc;
            stdenvName = p.stdenv.name or null;
            prevStdenvName =
              if (p.stdenv.__bootPackages or null) != null && (p.stdenv.__bootPackages.stdenv or null) != null then
                p.stdenv.__bootPackages.stdenv.name or null
              else
                null;
          }
        ]
      ) p.stdenv.__bootPackages;

  chain = walk [ ] pkgs;

  # Index from dfold side: reverse chain so index 0 = first dfold elem (reverseList head)
  chainByDfoldIndex = builtins.genList (i: builtins.elemAt chain ((builtins.length chain) - 1 - i)) (builtins.length chain);
in
{
  inherit chain chainByDfoldIndex;
  stageCount = builtins.length chain;
}
