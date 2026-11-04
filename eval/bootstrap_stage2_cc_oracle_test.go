package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestBootstrapStage2CCAttrOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "stage2_cc_oracle.nix")
	want := lay.EvalFileRaw(t, probe)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(probe)
	expr, err := parser.ParseFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("stage2 cc oracle\nnix=%s\ngo=%s", want, got)
	}
}
