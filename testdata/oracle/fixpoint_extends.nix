# PASS in real Nix: lib.extends overlay stack + lib.fix (nixpkgs-style).
{ libPath }:
let
  lib = import libPath;
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { fetchurl = 1; })
    (self: super: {
      probe = lib.intersectAttrs { fetchurl = false; } self;
    })
  ];
  pkgs = lib.fix toFix;
in pkgs.probe
