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
  attrs = stage2.drvAttrs;
in
builtins.toJSON {
  structuredAttrs = attrs.__structuredAttrs or false;
  drvAttrsHasDnbi = attrs ? defaultNativeBuildInputs;
}
