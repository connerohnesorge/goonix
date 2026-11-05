package eval_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBuiltinNamesMatchNix224(t *testing.T) {
	ver, err := exec.Command("nix-instantiate", "--version").CombinedOutput()
	if err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
	if !strings.Contains(string(ver), "2.24.9") {
		t.Skip("builtin name list is pinned to Nix 2.24.9")
	}
	lay := oracle.MustLayout(t)
	expr := `builtins.concatStringsSep "," (builtins.sort (a: b: a < b) (builtins.attrNames builtins))`
	wantRaw := lay.EvalRaw(t, expr)
	gotRaw := evalPkgsExprString(t, expr)
	if wantRaw != gotRaw {
		t.Fatalf("builtins attrNames mismatch\nnix=%q\ngo=%q", wantRaw, gotRaw)
	}
}
