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
  head = builtins.head stage1.defaultNativeBuildInputs;
in
builtins.toJSON {
  headOutPath = head.outPath;
  headToString = builtins.toString head;
  headType = head.type or null;
}
