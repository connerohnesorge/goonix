package eval

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestQuickOverridesHasAttr(t *testing.T) {
	root := testModRoot(t)
	src := `let pkgs = import "` + filepath.Join(root, "testdata/nixpkgs/default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in { hasOv = pkgs.stdenv ? overrides; tryPkgs = (builtins.tryEval pkgs).success; }`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + filepath.Join(root, "testdata/nixpkgs"))
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata/nix"))
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
		t.Fatalf("force: %v", err)
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
		t.Logf("%s=%v", name, val.DisplayString(ev, ev.Intern))
		if name == "tryPkgs" && (val.Kind != VKBool || !val.Bool) {
			t.Fatalf("tryEval pkgs should succeed like Nix")
		}
		if name == "hasOv" && (val.Kind != VKBool || !val.Bool) {
			t.Fatalf("final stdenv should have overrides attr")
		}
	}
}
