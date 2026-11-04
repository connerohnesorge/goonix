package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestPkgsXorgIsAttrset(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(nixpkgs); err != nil {
		t.Skip("no nixpkgs")
	}
	src := `
let pkgs = import "` + nixpkgs + `" {
  system = "x86_64-linux";
  config = { };
  overlays = [ ];
};
in pkgs.xorg
`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKAttrs {
		t.Fatalf("expected attrset, got %s", fv.DisplayString(ev, ev.Intern))
	}
}

func modRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		p := filepath.Dir(wd)
		if p == wd {
			t.Fatal("no go.mod")
		}
		wd = p
	}
}
