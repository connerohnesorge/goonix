# Cheap head probes on full nixpkgs import (no deep attr forcing).
{ nixpkgsPath ? ../nixpkgs }:
let
  pkgs =
    import (nixpkgsPath + "/default.nix") {
      localSystem = { system = "x86_64-linux"; };
      config = { };
      overlays = [ ];
    };
in
{
  tryPkgs = (builtins.tryEval pkgs).success;
  hasOverrides = pkgs.stdenv ? overrides;
  ccType = builtins.typeOf pkgs.stdenv.cc;
  tryStdenv = (builtins.tryEval pkgs.stdenv).success;
}
