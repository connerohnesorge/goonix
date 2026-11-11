package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestImportLibIsAttrsOracle(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `builtins.isAttrs (import "` + libPath + `")`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("expected true, got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestImportLibWHNFTypeIsSetOracle(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `builtins.typeOf (import "` + libPath + `") == "set"`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("expected true, got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestImportLibTrivialIdOracle(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `(import "` + libPath + `").trivial.id 1 == 1`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("expected true, got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestFixInheritSelfPathWHNF(t *testing.T) {
	src := `
let
  lib = { fix = f: let x = f x; in x; };
  val = lib.fix (self: {
    inherit (self.t) a;
    t = { a = 1; };
  });
in val.a
`
	ev := eval.NewEvaluator()
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
	if fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("expected 1, got %v", fv.DisplayString(ev, ev.Intern))
	}
}
