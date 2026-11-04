# PASS in real Nix: LHS of // is (f self), not bare self; RHS supplies callPackage for `with self`.
let
  f = self: with self; { got = callPackage; };
  self = f self // { callPackage = 7; };
in self.got
