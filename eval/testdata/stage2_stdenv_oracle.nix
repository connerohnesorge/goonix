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
  enbi = stage2.extraNativeBuildInputs or [ ];
in
builtins.toJSON {
  stdenvName = stage2.name;
  structuredAttrs = stage2.drvAttrs.__structuredAttrs or false;
  ccIsNull = cc == null;
  ccName = if cc == null then null else (cc.name or null);
  hasCC = stage2.hasCC or null;
  dnbiLen = builtins.length dnbi;
  dnbiNames = map (x: x.name or "?") dnbi;
  enbiLen = builtins.length enbi;
  enbiNames = map (x: x.name or "?") enbi;
  ccInDnbi = map (x: x.name or "?") (pkgs.lib.filter (x: x == cc) dnbi);
  gccWrapperCount = builtins.length (
    pkgs.lib.filter (n: pkgs.lib.hasInfix "gcc-wrapper" n) (map (x: x.name or "?") dnbi)
  );
}
