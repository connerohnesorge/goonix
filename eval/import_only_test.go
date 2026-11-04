package eval_test

import (
	"path/filepath"
	"testing"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestImportNixpkgsOnly(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `import "` + filepath.Join(nixpkgs, "default.nix") + `" { system = "x86_64-linux"; config = { }; overlays = [ ]; }`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, _ := parser.ParseString(src)
	_, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
