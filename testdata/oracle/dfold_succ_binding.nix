# Detect dfold off-by-one: tag whether succ used cur vs pred in inner go.
{ libPath ? ../nixpkgs/lib }:
let
  lib = import libPath;
  tags = [ "A" "B" "C" ];
  mkOp = succUsesCur:
    next: f: pred:
    {
      inherit (f) tag;
      predTag = pred.tag or "rnul";
      succTag = next.tag or "none";
      succUsesCur = succUsesCur;
    };

  dfoldCorrect = op: lnul: rnul: list:
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

  dfoldWrongSucc = op: lnul: rnul: list:
    let
      len = builtins.length list;
      go =
        pred: n:
        if n == len then
          rnul pred
        else
          let
            cur = op pred (builtins.elemAt list n) succ;
            succ = go pred (n + 1);
          in
            cur;
      lapp = lnul cur;
      cur = go lapp 0;
    in
      cur;

  lnul = _: { tag = "lnul"; };
  rnul = pred: { tag = "rnul"; predTag = pred.tag or "none"; };
  list = [
    { tag = "A"; }
    { tag = "B"; }
    { tag = "C"; }
  ];

  op = mkOp true;
  correct = dfoldCorrect op lnul rnul list;

  opW = mkOp false;
  wrong = dfoldWrongSucc opW lnul rnul list;
in
{
  correctSuccTag = correct.succTag;
  correctPredTag = correct.predTag;
  wrongSuccTag = wrong.succTag;
  wrongPredTag = wrong.predTag;
}
