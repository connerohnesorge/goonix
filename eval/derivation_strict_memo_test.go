package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestDerivationStrictMemoVerifyDuringBootstrapProbe(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	probe := filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix")
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{}).WithVerifyStrictMemo(true)
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	ev.SetCurFile(probe)
	expr, err := parser.ParseFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ev.EvalWithBuiltins(expr); err != nil {
		t.Fatal(err)
	}
}
