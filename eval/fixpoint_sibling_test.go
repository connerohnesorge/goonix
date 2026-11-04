package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFixpointIntersectAttrsSeesPublishedBinds(t *testing.T) {
	src := `
let
  lib = import "` + filepath.Join("testdata", "nixpkgs", "lib", "default.nix") + `";
  f = final: final // { fetchurl = 42; other = 1; };
  pkgs = lib.fix f;
in builtins.intersectAttrs { fetchurl = false; other = false; } pkgs
`
	root := modRoot(t)
	src = fmt.Sprintf(`
let
  lib = import %q;
  f = final: final // { fetchurl = 42; other = 1; };
  pkgs = lib.fix f;
in builtins.intersectAttrs { fetchurl = false; other = false; } pkgs
`, filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix"))
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
	if fv.Kind != eval.VKAttrs {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
	for _, name := range []string{"fetchurl", "other"} {
		sym := ev.Intern.Intern(name)
		if fv.Attrs.Binds[sym] == nil {
			t.Fatalf("missing %s in %v", name, fv.DisplayString(ev, ev.Intern))
		}
	}
}

func TestRecSiblingVisibleDuringForce(t *testing.T) {
	src := `
rec {
  a = b + 1;
  b = 10;
}
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
	sym := ev.Intern.Intern("a")
	b := fv.Attrs.Binds[sym]
	if b == nil {
		t.Fatalf("missing a in %v", fv.DisplayString(ev, ev.Intern))
	}
	var av *eval.Value
	if b.Thunk != nil {
		av, err = ev.ForceValue(eval.Deferred(b.Thunk))
	} else {
		av, err = ev.ForceValue(b.Value)
	}
	if err != nil {
		t.Fatal(err)
	}
	if av == nil || av.Kind != eval.VKInt || av.Int != 11 {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
