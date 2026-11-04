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
  attrs = gcc.drvAttrs;
  ty =
    name:
    if attrs ? ${name} then
      builtins.typeOf attrs.${name}
    else
      "absent";
in
builtins.toJSON {
  gccName = gcc.name;
  hasLibcCross = attrs ? libcCross;
  hasTargetConfig = attrs ? targetConfig;
  ignoreNulls = attrs.__ignoreNulls or false;
  structuredAttrs = attrs.__structuredAttrs or false;
  libcCrossType = ty "libcCross";
  targetConfigType = ty "targetConfig";
}
