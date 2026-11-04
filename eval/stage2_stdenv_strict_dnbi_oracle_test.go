package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBootstrapStage2StdenvStrictDNBIOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "stage2_stdenv_strict_dnbi_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("stage2 strict defaultNativeBuildInputs env oracle\nnix=%s\ngo=%s", want, got)
	}
}
