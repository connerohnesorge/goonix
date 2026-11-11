package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestImportNixpkgsLibVersionOnly(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + filepath.Join(nixpkgs, "default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in pkgs.lib.version`
	expr, _ := parser.ParseString(src)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	if s := forceStringOrPath(t, ev, val); s == "" {
		t.Fatal("empty version")
	}
}
