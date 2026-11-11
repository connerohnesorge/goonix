package eval

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestImportNixpkgsNoStoreCopy(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `import "` + filepath.Join(nixpkgs, "default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	if len(ev.copyPathTrace) != 0 {
		for i, e := range ev.copyPathTrace {
			if i >= 20 {
				t.Logf("... and %d more copies", len(ev.copyPathTrace)-20)
				break
			}
			t.Logf("copy[%d] reason=%s src=%q at %s:%d:%d", i, e.Reason, e.SourcePath, e.Pos.File, e.Pos.Line, e.Pos.Col)
		}
		t.Fatalf("expected no CopyPathToStore during nixpkgs import, got %d", len(ev.copyPathTrace))
	}
}
