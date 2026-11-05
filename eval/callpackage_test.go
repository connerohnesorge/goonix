package eval_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestMakeOverridableTwoStageFunction(t *testing.T) {
	root := modRoot(t)
	libNix := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  pkg = a: b: { va = a; vb = b; };
  r = lib.makeOverridable pkg { a = "A"; };
in {
  tyR = builtins.typeOf r;
  tyApply = builtins.typeOf (r "B");
}
`, libNix)
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
	for _, k := range []string{"tyR", "tyApply"} {
		sym := ev.Intern.Intern(k)
		b := fv.Attrs.Binds[sym]
		if b == nil {
			t.Fatalf("%s missing", k)
		}
		var sv *eval.Value
		if b.Thunk != nil {
			sv, err = ev.ForceValue(eval.Deferred(b.Thunk))
		} else {
			sv = b.Value
		}
		if err != nil {
			t.Fatal(err)
		}
		sv, err = ev.ForceValue(sv)
		if err != nil {
			t.Fatal(err)
		}
		if sv.Kind != eval.VKString {
			t.Fatalf("%s=%v", k, sv.DisplayString(ev, ev.Intern))
		}
		if k == "tyR" && sv.String != "set" && sv.String != "lambda" {
			t.Fatalf("tyR=%q want set or lambda", sv.String)
		}
		if k == "tyApply" && sv.String != "set" {
			t.Fatalf("tyApply=%q want set", sv.String)
		}
	}
}

func TestCallPackageWithXorgDefaultSelfCallPackage(t *testing.T) {
	testCallPackageWithTwoStageImport(t)
}

func TestCallPackageWithTwoStageImport(t *testing.T) {
	testCallPackageWithTwoStageImport(t)
}

func testCallPackageWithTwoStageImport(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	defNix := filepath.Join(nixpkgs, "pkgs", "servers", "x11", "xorg", "default.nix")
	src := fmt.Sprintf(`
let
  lib = import %q;
  auto = { inherit (lib) lib; pixman = null; };
  pkgFn = import %q;
  step1 = pkgFn auto;
  r = lib.callPackageWith auto pkgFn { };
in {
  tyStep1 = builtins.typeOf step1;
  isFn = lib.isFunction r;
  ty = builtins.typeOf r;
}
`, libNix, defNix)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
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
	forceStr := func(name string) string {
		b := fv.Attrs.Binds[ev.Intern.Intern(name)]
		if b == nil {
			t.Fatalf("missing %s", name)
		}
		var v *eval.Value
		if b.Thunk != nil {
			v, err = ev.ForceValue(eval.Deferred(b.Thunk))
		} else {
			v = b.Value
		}
		if err != nil {
			t.Fatal(err)
		}
		v, err = ev.ForceValue(v)
		if err != nil {
			t.Fatal(err)
		}
		if v.Kind != eval.VKString {
			t.Fatalf("%s=%v", name, v.DisplayString(ev, ev.Intern))
		}
		return v.String
	}
	if ty := forceStr("tyStep1"); ty != "lambda" {
		t.Fatalf("tyStep1=%q want lambda", ty)
	}
	isFnB := fv.Attrs.Binds[ev.Intern.Intern("isFn")]
	var isFnV *eval.Value
	if isFnB.Thunk != nil {
		isFnV, err = ev.ForceValue(eval.Deferred(isFnB.Thunk))
	} else {
		isFnV = isFnB.Value
	}
	if err != nil {
		t.Fatal(err)
	}
	isFnV, err = ev.ForceValue(isFnV)
	if err != nil {
		t.Fatal(err)
	}
	if isFnV.Kind != eval.VKBool || !isFnV.Bool {
		t.Fatalf("isFunction r false, ty=%q", forceStr("ty"))
	}
}
