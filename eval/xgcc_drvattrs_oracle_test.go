package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBootstrapXgccDrvAttrsOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "xgcc_drvattrs_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("xgcc drvAttrs oracle\nnix=%s\ngo=%s", want, got)
	}
}
