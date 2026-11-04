let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  findStage1Stdenv = st:
    if (st.name or "") == "bootstrap-stage1-stdenv-linux" then
      st
    else if st ? __bootPackages then
      findStage1Stdenv st.__bootPackages.stdenv
    else
      throw "bootstrap-stage1 stdenv not found (at ${st.name or "?"})";
  stage1 = findStage1Stdenv pkgs.stdenv;
  cc = stage1.cc;
  dnbi = stage1.defaultNativeBuildInputs;
in
builtins.toJSON {
  stdenvName = stage1.name;
  ccIsNull = cc == null;
  ccHasOutPath = cc ? outPath;
  ccName = if cc == null then null else (cc.name or null);
  hasCC = stage1.hasCC or null;
  dnbiLen = builtins.length dnbi;
  dnbiNames = map (x: x.name or "?") dnbi;
  preHookLen = builtins.stringLength stage1.preHook;
  preHookLines = builtins.length (pkgs.lib.splitString "\n" stage1.preHook);
}
