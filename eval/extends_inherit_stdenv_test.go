package eval

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestExtendsInheritStdenvDoesNotForceGccStdenv(t *testing.T) {
	root := testModRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib")
	src := `
let
  lib = import "` + libPath + `";
  toFix = lib.foldl' (lib.flip lib.extends) (self: { }) [
    (self: super: { stdenv = { cc = { isGNU = true; }; name = "boot"; }; })
    (self: super:
      let _ = self.stdenv.name;
      in { marker = 1; }
    )
    (self: super:
      let pkgs = self;
      in with pkgs; {
        gccStdenv = if stdenv.cc.isGNU then stdenv else null;
      }
    )
  ];
  pkgs = lib.fix toFix;
in {
  stdenvType = builtins.typeOf pkgs.stdenv;
  ccType = builtins.typeOf pkgs.stdenv.cc;
}
`
	ev := NewEvaluator()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.forceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	for sym, bind := range fv.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		var raw *Value
		if bind.Thunk != nil {
			raw = Deferred(bind.Thunk)
		} else {
			raw = bind.Value
		}
		val, err := ev.forceValue(raw)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		t.Logf("%s=%s", name, val.DisplayString(ev, ev.Intern))
	}
}
