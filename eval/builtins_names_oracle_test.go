package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBuiltinNamesMatchNix224(t *testing.T) {
	oracle.RequireNix224(t)
	lay := oracle.MustLayout(t)
	expr := `builtins.concatStringsSep "," (builtins.sort (a: b: a < b) (builtins.attrNames builtins))`
	wantRaw := lay.EvalRaw(t, expr)
	gotRaw := evalPkgsExprString(t, expr)
	if wantRaw != gotRaw {
		t.Fatalf("builtins attrNames mismatch\nnix=%q\ngo=%q", wantRaw, gotRaw)
	}
}
