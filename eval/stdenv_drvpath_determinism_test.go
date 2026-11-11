package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestStdenvDrvPathTwiceSameEvaluator(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in { a = pkgs.stdenv.drvPath; b = pkgs.stdenv.drvPath; }`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	get := func(name string) string {
		t.Helper()
		b := fv.Attrs.Binds[ev.Intern.Intern(name)]
		if b == nil {
			t.Fatalf("missing %s", name)
		}
		v, err := ev.ForceAttrBinding(b)
		if err != nil {
			t.Fatal(err)
		}
		s, err := forceStringOrPathEvalTest(t, ev, v)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	ap := get("a")
	bp := get("b")
	if ap != bp {
		t.Fatalf("drvPath differs in one evaluator: %q vs %q", ap, bp)
	}
}

func TestStdenvDrvPathTwoFreshEvaluators(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + nixpkgs + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in pkgs.stdenv.drvPath`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	evalOnce := func() string {
		ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
		ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
		ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
		val, err := ev.EvalWithBuiltins(expr)
		if err != nil {
			t.Fatal(err)
		}
		return forceStringOrPath(t, ev, val)
	}
	p1 := evalOnce()
	p2 := evalOnce()
	if p1 != p2 {
		t.Fatalf("drvPath differs across evaluators: %q vs %q", p1, p2)
	}
}
