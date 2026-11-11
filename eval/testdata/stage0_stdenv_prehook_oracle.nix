let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  findStage0Stdenv = st:
    if (st.name or "") == "bootstrap-stage0-stdenv-linux" then
      st
    else if st ? __bootPackages then
      findStage0Stdenv st.__bootPackages.stdenv
    else
      throw "bootstrap-stage0 stdenv not found (at ${st.name or "?"})";
  stage0 = findStage0Stdenv pkgs.stdenv;
in
builtins.toJSON {
  stdenvName = stage0.name;
  preHookLen = builtins.stringLength stage0.preHook;
  preHookLines = builtins.length (pkgs.lib.splitString "\n" stage0.preHook);
}
