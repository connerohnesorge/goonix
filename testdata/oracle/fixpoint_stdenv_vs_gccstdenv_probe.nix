# Minimal fixpoint: does forcing pkgs.stdenv evaluate gccStdenv (all-packages shape)?
{ libPath ? ../nixpkgs/lib }:
let
  lib = import libPath;
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: {
      stdenv = {
        cc = { isGNU = true; };
        hasCC = true;
        overrides = self: super: { bootGlibc = "from-overrides"; };
      };
    })
    (self: super:
      let
        pkgs = self;
      in
      with pkgs; {
        gccStdenv = if stdenv.cc.isGNU then stdenv else null;
      })
    (self: super: (super.stdenv.overrides or (_: _: { })) self super)
  ];
  pkgs = lib.fix toFix;
in
{
  tryPkgs = (builtins.tryEval pkgs).success;
  tryStdenv = (builtins.tryEval pkgs.stdenv).success;
  stdenvMarker = pkgs.stdenv.cc.isGNU or false;
}
