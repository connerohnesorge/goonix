package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBootstrapXgccLibraryPathGetContextOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "xgcc_library_path_getcontext_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("LIBRARY_PATH getContext oracle\nnix=%s\ngo=%s", want, got)
	}
}
