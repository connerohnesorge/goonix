package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBootstrapXgccNestedEnvGetContextOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "xgcc_nested_env_getcontext_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("nested env getContext oracle\nnix=%s\ngo=%s", want, got)
	}
}
