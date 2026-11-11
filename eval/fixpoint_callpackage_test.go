package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFixpointExtendsIntersectAttrsSeesFetchurl(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { fetchurl = 1; })
    (self: super: {
      xorgProbe = lib.intersectAttrs { fetchurl = false; } self;
    })
  ];
  pkgs = lib.fix toFix;
in pkgs.xorgProbe
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
	sym := ev.Intern.Intern("fetchurl")
	if fv.Kind != eval.VKAttrs || fv.Attrs.Binds[sym] == nil {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}
