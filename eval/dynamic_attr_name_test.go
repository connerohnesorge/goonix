package eval_test

import "testing"

func TestDynamicAttrNameFromSetMatchesNix(t *testing.T) {
	src := `
let
  localSystem = { libc = "glibc"; };
  overrides = self: super: { ${localSystem.libc} = 42; };
  r = overrides { } { };
in { v = r.glibc; }
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("dynamic attr:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
