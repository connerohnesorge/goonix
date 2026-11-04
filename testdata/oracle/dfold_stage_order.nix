# Reproduce booter dfold stage wiring without full nixpkgs.
# Each stageFun tags prevStage.stdenv.name so we can see dfold pred/succ order.
{ libPath ? ../nixpkgs/lib, allPackages ? (args: args // { tag = "allPackages-" + (args.stdenv.name or "?"); }) }:
let
  lib = import libPath;

  stageFuns = [
    (prev: { stdenv = { name = "stage-A"; cc = null; }; tag = "fun-0"; prevSeen = prev.stdenv.name or "null-prev"; })
    (prev: { stdenv = { name = "stage-B"; cc = { }; }; tag = "fun-1"; prevSeen = prev.stdenv.name or "null-prev"; })
    (prev: { stdenv = { name = "stage-C"; cc = { }; }; tag = "fun-2"; prevSeen = prev.stdenv.name or "null-prev"; })
  ];

  dfold = op: lnul: rnul: list:
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
      thisStage =
        if args.__raw or false then
          args'
        else
          allPackages args';
    in
      thisStage;

  postStage = buildPackages: { __raw = true; tag = "postStage"; };

  result = dfold folder postStage (_: { tag = "rnul"; }) withAllowCustomOverrides;

  # Walk __bootPackages chain from final result
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

  # dfold list order metadata (without forcing stages)
  reversedNames = map (f: (f { stdenv = { name = "probe-null"; }; }).stdenv.name or "x") (lib.lists.reverseList stageFuns);

in
{
  inherit chain reversedNames;
  finalStdenvName = result.stdenv.name or null;
  finalCcType = builtins.typeOf result.stdenv.cc;
}
