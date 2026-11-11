{ nixpkgs ? ./nixpkgs }:
let
  lib = import (nixpkgs + "/lib");
  auto = {
    inherit (lib) lib;
    pixman = null;
    fetchurl = lib.trivial.id;
  };
  def = import (nixpkgs + "/pkgs/servers/x11/xorg/default.nix");
  generatedPackages = lib.callPackageWith auto def { };
  scope = lib.makeScopeWithSplicing' {
    splicePackages = x: x;
    newScope = s: x: x // s;
  } {
    otherSplices = {
      selfBuildBuild = { };
      selfBuildHost = { };
      selfBuildTarget = { };
      selfHostHost = { };
      selfHostTarget = { };
      selfTargetTarget = { };
    };
    f = lib.extends (_: _: { }) generatedPackages;
  };
in
scope.fontadobeutopiatype1
