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

func TestStringInterpPathGetContextMatchesNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	dir := filepath.Join(root, "eval", "testdata")
	x := filepath.Join(dir, "path_interp_x.txt")
	if err := os.WriteFile(x, []byte("xy"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(x) })

	// Path literal (not quoted string) so Nix copies in "${p}" string context.
	nixPath := x
	if !strings.HasPrefix(nixPath, "/") {
		t.Fatal("need absolute path for nix path literal")
	}
	lay := oracle.MustLayout(t)
	src := `let p = ` + nixPath + `; s = "${p}"; in builtins.toJSON { ctx = builtins.getContext s; val = s; literal = builtins.toString p; litCtx = builtins.getContext (builtins.toString p); }`
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
		t.Fatalf("path string context oracle mismatch\nnix=%s\ngo=%s", want, got)
	}
	if !strings.Contains(got, "/nix/store/") {
		t.Fatalf("expected store path in interpolated string JSON: %s", got)
	}
}

func TestStringPlusPathCopiesMatchesNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	x := filepath.Join(root, "eval", "testdata", "path_plus_y.txt")
	if err := os.WriteFile(x, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(x) })

	lay := oracle.MustLayout(t)
	src := `let p = ` + x + `; s = "pre-" + p; in builtins.toJSON (builtins.getContext s)`
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
		t.Fatalf("string+path context\nnix=%s\ngo=%s", want, got)
	}
}
