# Minimal reproducers confirmed with Nix 2.24.x+ (see eval/oracle_repro_test.go).
#
# Bare `final // { … }` inside lib.fix is NOT valid in real Nix (LHS WHNF hits blackhole).
# nix-instantiate --eval testdata/oracle/fixpoint_bare_update.nix  → infinite recursion
#
# Productive patterns:
# - extends + lib.fix (fixpoint_extends.nix)
# - (f self) // { … } with `with self` (with_self_callpackage.nix)
# - import <nixpkgs> { config = { }; } (import_nixpkgs_empty_config.nix)
