# Bisect real linux boot via stdenv.__bootPackages (depth 0 = final import).
{ nixpkgsPath ? ../nixpkgs, maxDepth ? 8, pick ? null }:
let
  pkgs =
    import (nixpkgsPath + "/default.nix") {
      localSystem = { system = "x86_64-linux"; };
      config = { };
      overlays = [ ];
    };

  stageAt =
    depth:
    let
      loop =
        s: n:
        if n == 0 then
          s
        else
          let
            c = builtins.tryEval (s.stdenv.__bootPackages or null);
          in
          if c.success && c.value != null then loop c.value (n - 1) else null;
    in
    if depth == 0 then pkgs else loop pkgs depth;

  rowFor =
    depth: s:
    if s == null then
      {
        depth = depth;
        stageTryOk = false;
        stdenvTryOk = false;
        ccType = "absent";
        stdenvName = null;
        hasOverridesAttr = false;
        overrideNames = [ ];
        overrideNamesTryOk = false;
        glibcSameAsChild = null;
        childTryOk = false;
      }
    else
      let
        sTry = builtins.tryEval s;
        child =
          if sTry.success && (s ? stdenv) then
            builtins.tryEval (s.stdenv.__bootPackages or null)
          else
            { success = false; value = null; };
        childVal = if child.success then child.value else null;

        stdenvTry =
          if sTry.success && (s ? stdenv) then builtins.tryEval s.stdenv else { success = false; value = null; };

        ccTypeTry =
          if stdenvTry.success then
            builtins.tryEval (builtins.typeOf s.stdenv.cc)
          else
            { success = false; value = "no-stdenv"; };

        stdenvNameTry =
          if stdenvTry.success then builtins.tryEval (s.stdenv.name or null) else { success = false; value = null; };

        hasOvTry =
          if stdenvTry.success then builtins.tryEval (s.stdenv ? overrides) else { success = false; value = false; };

        overrideNamesTry =
          if stdenvTry.success && hasOvTry.success && hasOvTry.value then
            let
              ov = s.stdenv.overrides or (_: _: { });
            in
            if builtins.isFunction ov then
              builtins.tryEval (builtins.attrNames (ov s s))
            else
              { success = false; value = [ ]; }
          else
            { success = false; value = [ ]; };

        glibcHereTry = if sTry.success && (s ? glibc) then builtins.tryEval s.glibc else { success = false; };
        glibcChildTry =
          if childVal != null && (childVal ? glibc) then
            builtins.tryEval childVal.glibc
          else
            { success = false; };

        glibcSameAsChild =
          if glibcHereTry.success && glibcChildTry.success then
            glibcHereTry.value.drvPath == glibcChildTry.value.drvPath
          else
            null;

      in
      {
        inherit depth;
        stageTryOk = sTry.success;
        stdenvTryOk = stdenvTry.success;
        ccType = if ccTypeTry.success then ccTypeTry.value else "error";
        stdenvName = if stdenvNameTry.success then stdenvNameTry.value else null;
        hasOverridesAttr = if hasOvTry.success then hasOvTry.value else false;
        overrideNames = if overrideNamesTry.success then overrideNamesTry.value else [ ];
        overrideNamesTryOk = overrideNamesTry.success;
        inherit glibcSameAsChild;
        childTryOk = child.success;
      };

  chain = builtins.genList (d: rowFor d (stageAt d)) maxDepth;

  effectiveCount =
    let
      countOk =
        n:
        if n >= maxDepth then maxDepth
        else if (builtins.elemAt chain n).stageTryOk then countOk (n + 1) else n;
    in
    countOk 0;

in
if pick != null then
  rowFor pick (stageAt pick)
else
  {
    inherit chain effectiveCount;
    stageCount = effectiveCount;
  }
