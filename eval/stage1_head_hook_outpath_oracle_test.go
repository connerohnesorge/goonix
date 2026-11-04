package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBootstrapStage1HeadHookOutPathOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "stage1_head_hook_outpath_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("stage1 head hook outPath oracle\nnix=%s\ngo=%s", want, got)
	}
}
