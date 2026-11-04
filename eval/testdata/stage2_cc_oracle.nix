let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  findStage2Stdenv = st:
    if (st.name or "") == "bootstrap-stage2-stdenv-linux" then
      st
    else if st ? __bootPackages then
      findStage2Stdenv st.__bootPackages.stdenv
    else
      throw "bootstrap-stage2 stdenv not found (at ${st.name or "?"})";
  stage2 = findStage2Stdenv pkgs.stdenv;
  cc = stage2.cc;
  dnbi = stage2.defaultNativeBuildInputs;
  n = builtins.length dnbi;
  last = if n > 0 then builtins.elemAt dnbi (n - 1) else null;
in
builtins.toJSON {
  stdenvName = stage2.name;
  ccHasOutPath = cc ? outPath;
  ccAttrNames = builtins.attrNames cc;
  ccOutPathTryOk = (builtins.tryEval cc.outPath).success;
  ccType = if cc ? type then cc.type else null;
  dnbiLen = n;
  lastHasOutPath = if last != null then last ? outPath else false;
  lastOutPathTryOk = if last != null then (builtins.tryEval last.outPath).success else false;
  lastSameAsCC = if last != null then last.outPath == cc.outPath else false;
  lastToStringLen = if last != null then builtins.stringLength (builtins.toString last) else 0;
}
