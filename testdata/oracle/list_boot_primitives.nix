# Primitive list/boot helpers used by booter.nix (compare goonix vs Nix JSON).
{ libPath ? ../nixpkgs/lib }:
let
  lib = import libPath;
  stageFuns = [
    (_: { n = "s0"; })
    (_: { n = "s1"; })
    (_: { n = "s2"; })
    (_: { n = "s3"; })
  ];
  xs = [ "a" "b" "c" "d" "e" ];
in
{
  lengthStages = builtins.length stageFuns;
  reverseListStages = map (f: (f { }).n) (lib.lists.reverseList stageFuns);
  imap1Indexes = lib.lists.imap1 (i: v: i) stageFuns;
  imap1OnReversed = lib.lists.imap1 (i: v: i) (lib.lists.reverseList stageFuns);
  genListIds = builtins.genList (i: i) (builtins.length stageFuns);
  elemAt0Type = builtins.typeOf (builtins.elemAt stageFuns 0);
  elemAtLastN = (builtins.elemAt stageFuns (builtins.length stageFuns - 1) { }).n;
  initLen = builtins.length (lib.lists.init xs);
  lastVal = lib.lists.last xs;
  foldlPrime = builtins.foldl' (acc: el: acc + el) 0 (builtins.genList (i: i + 1) 5);
  append = [ 1 ] ++ [ 2 3 ];
  orFalse = (null).isClang or false;
  orTrue = ({ isClang = true; }).isClang or false;
  dfoldLen = builtins.length (lib.lists.imap1 (a: b: c: null) (lib.lists.reverseList stageFuns));
}
