let
  pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
  tryXgccGU =
    st:
    let
      bp = st.__bootPackages or null;
      gu = if bp != null then bp.gcc-unwrapped or null else null;
    in
    if gu != null && builtins.match "xgcc-.*" (gu.name or "") != null then gu else null;
  walk =
    st:
    let a = tryXgccGU st;
    in if a != null then a else if st ? __bootPackages then walk st.__bootPackages.stdenv else null;
  gcc = walk pkgs.stdenv;
  s = gcc.drvAttrs.LIBRARY_PATH or "";
  ctx = builtins.getContext s;
in
builtins.toJSON {
  libraryPath = s;
  contextKeys = builtins.attrNames ctx;
  context = ctx;
}
