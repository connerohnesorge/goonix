package eval_test

import "testing"

func TestNullDynamicAttrNameDroppedMatchesNix(t *testing.T) {
	src := `
let
  r = { ${if false then "CC" else null} = "gcc -m64"; other = 1; };
in { names = builtins.attrNames r; }
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("null dynamic attr name:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
