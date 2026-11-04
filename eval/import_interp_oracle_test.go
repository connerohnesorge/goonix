package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestStringInterpInsideImportedModuleMatchesNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	mod := filepath.Join(root, "eval", "testdata", "import_interp", "mod.nix")
	lay := oracle.MustLayout(t)
	src := `let m = import "` + mod + `"; in builtins.toJSON { ctx = builtins.getContext m.s; val = m.s; }`
	want := lay.EvalRaw(t, src)

	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("imported module string interp\nnix=%s\ngo=%s", want, got)
	}
	if !strings.Contains(got, "/nix/store/") {
		t.Fatalf("expected store path in JSON: %s", got)
	}
}
