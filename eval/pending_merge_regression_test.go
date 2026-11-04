package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// TestPendingMergeWithSelfAndFixpointIntersectAttrs guards two separate laziness paths:
// (1) with self sees callPackage on pending // RHS while self = f self // { … } forces;
// (2) lib.fix sibling names (fetchurl) visible to intersectAttrs / callPackageWith auto pool.
func TestPendingMergeWithSelfAndFixpointIntersectAttrs(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")

	withSelf := `
let
  f = self: with self; { got = callPackage; };
  self = f self // { callPackage = 7; };
in self.got
`
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(withSelf)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatalf("with-self pending merge: %v", err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKInt || fv.Int != 7 {
		t.Fatalf("with-self pending merge: got %v", fv.DisplayString(ev, ev.Intern))
	}

	fixSrc := fmt.Sprintf(`
let
  lib = import %q;
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { fetchurl = 1; })
    (self: super: {
      probe = lib.intersectAttrs { fetchurl = false; } self;
    })
  ];
  pkgs = lib.fix toFix;
in pkgs.probe
`, libPath)
	ev2 := eval.NewEvaluator()
	expr2, err := parser.ParseString(fixSrc)
	if err != nil {
		t.Fatal(err)
	}
	val2, err := ev2.EvalWithBuiltins(expr2)
	if err != nil {
		t.Fatalf("fixpoint intersectAttrs pool: %v", err)
	}
	fv2, err := ev2.ForceValue(val2)
	if err != nil {
		t.Fatal(err)
	}
	sym := ev2.Intern.Intern("fetchurl")
	if fv2.Kind != eval.VKAttrs || fv2.Attrs.Binds[sym] == nil {
		t.Fatalf("fixpoint intersectAttrs pool: got %v", fv2.DisplayString(ev2, ev2.Intern))
	}
}
