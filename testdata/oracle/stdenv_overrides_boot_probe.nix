# Final boot stage (+ immediate prev): stdenv.overrides and glibc vs prevStage.
{ nixpkgsPath ? ../nixpkgs }:
let
  pkgs =
    import (nixpkgsPath + "/default.nix") {
      localSystem = { system = "x86_64-linux"; };
      config = { };
      overlays = [ ];
    };

  prev = pkgs.stdenv.__bootPackages or null;

  rowFor = stage:
    let
      ov = stage.stdenv.overrides or (_: _: { });
      appliedTry = builtins.tryEval (ov stage stage);
    in
    {
      stdenvName = stage.stdenv.name or null;
      hasOverridesAttr = stage.stdenv ? overrides;
      overridesIsFunction = builtins.isFunction ov;
      overrideNames =
        if appliedTry.success then builtins.attrNames appliedTry.value else [ ];
      hasGlibcInOverrides = if appliedTry.success then appliedTry.value ? glibc else false;
    };

in
{
  tryPkgs = (builtins.tryEval pkgs).success;
  final = rowFor pkgs;
  prev = if prev != null && (prev ? stdenv) then rowFor prev else null;
  glibcSameAsPrev =
    if prev != null && (prev ? glibc) then
      (builtins.tryEval pkgs.glibc).success
      && (builtins.tryEval prev.glibc).success
      && pkgs.glibc.drvPath == prev.glibc.drvPath
    else
      false;
}
