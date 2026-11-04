package eval_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func forceAttrBool(t *testing.T, ev *eval.Evaluator, fv *eval.Value, name string) *eval.Value {
	t.Helper()
	sym := ev.Intern.Intern(name)
	b := fv.Attrs.Binds[sym]
	if b == nil {
		t.Fatalf("missing %s", name)
	}
	var v *eval.Value
	if b.Thunk != nil {
		var err error
		v, err = ev.ForceValue(eval.Deferred(b.Thunk))
		if err != nil {
			t.Fatal(err)
		}
	} else {
		v = b.Value
	}
	bv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	return bv
}

func TestDynamicAttrSelect(t *testing.T) {
	src := `let a = { "2" = "ok"; }; in a.${toString 2}`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKString || fv.String != "ok" {
		t.Fatalf("got %q", fv.DisplayString(ev, ev.Intern))
	}
}

func TestOptionalFormalNotForced(t *testing.T) {
	src := `
let
  f = { a, b ? (throw "default forced") }: if a ? b then "has" else "no";
in f { a = { x = 1; }; }
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
	fv, _ := ev.ForceValue(val)
	if fv.Kind != eval.VKString || fv.String != "no" {
		t.Fatalf("got %q", fv.DisplayString(ev, ev.Intern))
	}
}

func nixpkgsRoot(t *testing.T) (root, nixpkgs, libNix, parseNix string) {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}
	nixpkgs = filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(nixpkgs); err != nil {
		t.Skip("testdata/nixpkgs not present")
	}
	libNix = filepath.Join(nixpkgs, "lib", "default.nix")
	parseNix = filepath.Join(nixpkgs, "lib", "systems", "parse.nix")
	return root, nixpkgs, libNix, parseNix
}

func TestParsedPlatformCheckManual(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)

	src := fmt.Sprintf(`
let
  lib = import %q;
  parse = import %q { inherit lib; };
  abi = parse.abis.gnu;
  list = lib.attrValues parse.abis;
  head = builtins.head list;
in {
  abTy = builtins.typeOf parse.abis;
  firstValTy = builtins.typeOf (builtins.elemAt (lib.attrValues parse.abis) 0);
  gnuTy = builtins.typeOf parse.abis.gnu;
  eqGnu = builtins.elem parse.abis.gnu (lib.attrValues parse.abis);
}
`, libNix, parseNix)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"abTy", "firstValTy", "gnuTy", "eqGnu"} {
		bv := forceAttrBool(t, ev, fv, name)
		if name == "eqGnu" {
			if bv.Kind != eval.VKBool || !bv.Bool {
				t.Fatalf("eqGnu=false abTy=%v firstValTy=%v gnuTy=%v",
					forceAttrBool(t, ev, fv, "abTy").DisplayString(ev, ev.Intern),
					forceAttrBool(t, ev, fv, "firstValTy").DisplayString(ev, ev.Intern),
					forceAttrBool(t, ev, fv, "gnuTy").DisplayString(ev, ev.Intern))
			}
			continue
		}
		if bv.Kind != eval.VKString || bv.String != "set" {
			t.Fatalf("%s=%v", name, bv.DisplayString(ev, ev.Intern))
		}
	}
}

func TestParseMkSystemFromSkeletonDirect(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`
let
  lib = import %q;
  parse = import %q { inherit lib; };
  sk = { cpu = "x86_64"; kernel = "linux"; };
in parse.mkSystemFromSkeleton sk
`, libNix, parseNix)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	ev.ForceValue(val)
}

func TestMkSystemFromStringX8664Linux(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)

	src := fmt.Sprintf(`
let
  lib = import %q;
  parse = import %q { inherit lib; };
in parse.mkSystemFromString "x86_64-linux"
`, libNix, parseNix)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKAttrs {
		t.Fatalf("expected attrs, got %v", fv.DisplayString(ev, ev.Intern))
	}
	typeSym := ev.Intern.Intern("_type")
	bv := forceAttrBool(t, ev, fv, "_type")
	if bv.Kind != eval.VKString || bv.String != "system" {
		t.Fatalf("expected _type system, got %v", bv.DisplayString(ev, ev.Intern))
	}
	_ = typeSym
}
