package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestStdenvCcTypeAfterImport(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := fmt.Sprintf(`
let pkgs = import %q { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in {
  typeOf = builtins.typeOf pkgs.stdenv.cc;
  isNull = pkgs.stdenv.cc == null;
}
`, filepath.Join(nixpkgs, "default.nix"))
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatalf("eval (import must succeed for this probe): %v", err)
	}
}
