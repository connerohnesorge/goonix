let
  pkgs = import <nixpkgs> { config = { }; overlays = [ ]; };
  findStage2 =
    st:
    if (st.name or "") == "bootstrap-stage2-stdenv-linux" then
      st
    else if st ? __bootPackages then
      findStage2 st.__bootPackages.stdenv
    else
      throw "bootstrap-stage2 stdenv not found";
  st2 = findStage2 pkgs.stdenv;
  erp = st2.cc.expand-response-params;
  erpDrv =
    if erp == null || erp == "" then
      null
    else if builtins.isString erp then
      null
    else
      erp;
  drvText = if erpDrv != null then builtins.readFile erpDrv.drvPath else "";
  m = if drvText != "" then builtins.match ".*\"stdenv\";\"(/nix/store/[^\"]+)\"" drvText else null;
in
builtins.toJSON {
  erpIsDrv = erpDrv != null;
  stdenvStorePath = if m != null then builtins.elemAt m 0 else null;
  stdenvBase = if m != null then builtins.baseNameOf (builtins.elemAt m 0) else null;
}
