package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// Regression: rec { nestedTypes.elemType = elemType; } must not bind elemType at the
// rec top-level env (shadows lambda formals and causes infinite recursion).
func TestRecNestedPathDoesNotShadowOuter(t *testing.T) {
	src := `
let
  f = elemType: rec { nestedTypes.elemType = elemType; };
in (f "ok").nestedTypes.elemType
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	if val.Kind != eval.VKString || val.String != "ok" {
		t.Fatalf("got %v", val.DisplayString(ev, ev.Intern))
	}
}

func TestImportLibTypes(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}
	libNix := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	if _, err := os.Stat(libNix); err != nil {
		t.Skip("no nixpkgs")
	}
	ev := eval.NewEvaluator()
	ev.SetCurFile(libNix)
	expr, err := parser.ParseString(`import "` + libNix + `"`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
