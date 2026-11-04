# FAILS in real Nix: infinite recursion on LHS of // (oracle documents anti-pattern).
let
  fix = f: let x = f x; in x;
  f = final: final // { fetchurl = 42; other = 1; };
  pkgs = fix f;
in builtins.intersectAttrs { fetchurl = false; other = false; } pkgs
