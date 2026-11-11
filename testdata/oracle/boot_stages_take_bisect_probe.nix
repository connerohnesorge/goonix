# Bottom-up boot bisect: truncate linux stdenv stage list with lib.take k.
{ nixpkgsPath ? ../nixpkgs, maxK ? 11, pick ? null }:
let
  lib = import (nixpkgsPath + "/lib");
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  crossSystem = localSystem;
  config = { };
  overlays = [ ];
  crossOverlays = [ ];

  importPkgs =
    k:
    import (nixpkgsPath + "/default.nix") {
      inherit localSystem crossSystem config overlays crossOverlays;
      stdenvStages = args: lib.take k (import (nixpkgsPath + "/pkgs/stdenv") args);
    };

  # k=1 is only the __raw null stage; booter merge has no args.stdenv (Nix strict cannot tryEval pkgs.stdenv).
  rowDegenerate =
    k:
    let
      tryPkgs = builtins.tryEval (importPkgs k);
    in
    {
      inherit k;
      tryPkgs = tryPkgs.success;
      tryPkgsMsg = if tryPkgs.success then "" else tryPkgs.value;
      stdenvTryOk = false;
      ccType = "error";
      ccTypeTryOk = false;
      stdenvName = null;
      stdenvNameTryOk = false;
      hasOverridesAttr = false;
      hasOverridesTryOk = false;
      glibcDrvTryOk = false;
    };

  rowFor =
    k:
    if k == 1 then
      rowDegenerate k
    else
      let
        pkgs = importPkgs k;
        tryPkgs = builtins.tryEval pkgs;
        stdenvTry = if tryPkgs.success then builtins.tryEval pkgs.stdenv else { success = false; };
        ccTypeTry =
          if stdenvTry.success then
            builtins.tryEval (builtins.typeOf pkgs.stdenv.cc)
          else
            { success = false; };
        stdenvNameTry =
          if stdenvTry.success then
            builtins.tryEval (pkgs.stdenv.name or null)
          else
            { success = false; };
        hasOvTry =
          if stdenvTry.success then
            builtins.tryEval (pkgs.stdenv ? overrides)
          else
            { success = false; };
        glibcDrvTry =
          if tryPkgs.success then
            builtins.tryEval pkgs.glibc.drvPath
          else
            { success = false; };
      in
      {
        k = k;
        tryPkgs = tryPkgs.success;
        tryPkgsMsg = if tryPkgs.success then "" else tryPkgs.value;
        stdenvTryOk = stdenvTry.success;
        ccType = if ccTypeTry.success then ccTypeTry.value else "error";
        ccTypeTryOk = ccTypeTry.success;
        stdenvName = if stdenvNameTry.success then stdenvNameTry.value else null;
        stdenvNameTryOk = stdenvNameTry.success;
        hasOverridesAttr = if hasOvTry.success then hasOvTry.value else false;
        hasOverridesTryOk = hasOvTry.success;
        glibcDrvTryOk = glibcDrvTry.success;
      };

  rows = builtins.genList (i: rowFor (i + 1)) maxK;

in
if pick != null then
  rowFor pick
else
  {
    inherit maxK;
    stageCount = maxK;
    inherit rows;
  }
