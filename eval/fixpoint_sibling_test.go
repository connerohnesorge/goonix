package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// Bare `final // { … }` inside lib.fix is invalid in real Nix (see testdata/oracle/fixpoint_bare_update.nix).
// nixpkgs uses lib.extends overlays; this matches testdata/oracle/fixpoint_extends.nix.
func TestFixpointIntersectAttrsSeesPublishedBinds(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { fetchurl = 42; })
    (self: super: { other = 1; })
    (self: super: {
      probe = lib.intersectAttrs { fetchurl = false; other = false; } self;
    })
  ];
  pkgs = lib.fix toFix;
in pkgs.probe
`, libPath)
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
