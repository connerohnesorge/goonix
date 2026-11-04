# Nine-stage dfold wiring check (same count as linux boot list).
{ libPath ? ../nixpkgs/lib, allPackages ? (args: args // { tag = "allPackages-" + (args.stdenv.name or "?"); }) }:
let
  lib = import libPath;

  stageFuns = lib.genList (
    i:
    prev:
    {
      stdenv = {
        name = "stage-${toString i}";
        cc = if i == 0 then null else { };
      };
      tag = "fun-${toString i}";
      prevSeen = prev.stdenv.name or "null-prev";
    }
  ) 9;

  dfold =
    op: lnul: rnul: list:
    let
      len = builtins.length list;
      go =
        pred: n:
        if n == len then
          rnul pred
        else
          let
            cur = op pred (builtins.elemAt list n) succ;
            succ = go cur (n + 1);
          in
            cur;
      lapp = lnul cur;
      cur = go lapp 0;
    in
      cur;

  withAllowCustomOverrides = lib.lists.imap1 (
    index: stageFun: prevStage:
    { allowCustomOverrides = index == 1; } // (stageFun prevStage)
  ) (lib.lists.reverseList stageFuns);

  folder = nextStage: stageFun: prevStage:
    let
      args = stageFun prevStage;
      args' = args // {
        stdenv = args.stdenv // {
          __bootPackages = prevStage;
          __hatPackages = nextStage;
        };
      };
      thisStage = if args.__raw or false then args' else allPackages args';
    in
      thisStage;

  postStage = buildPackages: { __raw = true; tag = "postStage"; };

  result = dfold folder postStage (_: { tag = "rnul"; }) withAllowCustomOverrides;

  walk =
    acc: p:
    if !(p ? stdenv) then acc
    else
      walk (
        acc
        ++ [
          {
            ccType = builtins.typeOf p.stdenv.cc;
            stdenvName = p.stdenv.name or null;
            prevStdenvName =
              if (p.stdenv.__bootPackages or null) != null && (p.stdenv.__bootPackages.stdenv or null) != null then
                p.stdenv.__bootPackages.stdenv.name or null
              else
                null;
            stageTag = p.tag or null;
            prevSeen = p.prevSeen or null;
          }
        ]
      ) (p.stdenv.__bootPackages or { });

  chain = walk [ ] result;
  chainByDfoldIndex = builtins.genList (i: builtins.elemAt chain ((builtins.length chain) - 1 - i)) (builtins.length chain);
in
{
  inherit chain chainByDfoldIndex;
  finalStdenvName = result.stdenv.name or null;
  finalCcType = builtins.typeOf result.stdenv.cc;
}
