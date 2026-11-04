package eval_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestParseKernelsDarwinAlias(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; in parse.kernels.darwin`, libNix, parseNix)
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

func TestParseKernelsListForce(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; k = parse.kernels; in [ k.macos k.linux k.ios k.freebsd ]`, libNix, parseNix)
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

func TestParseKernelsIosWhole(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; in parse.kernels.ios`, libNix, parseNix)
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

func TestParseKernelsMacosWhole(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; in parse.kernels.macos`, libNix, parseNix)
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

func TestParseKernelsAttrValues(t *testing.T) {
	_, nixpkgs, libNix, parseNix := nixpkgsRoot(t)
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; in builtins.length (lib.attrValues parse.kernels)`, libNix, parseNix)
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

func TestParseKernelsMacosFamilies(t *testing.T) {
	root, err := os.Getwd()
	if err != nil { t.Fatal(err) }
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libNix := filepath.Join(nixpkgs, "lib", "default.nix")
	parseNix := filepath.Join(nixpkgs, "lib", "systems", "parse.nix")
	src := fmt.Sprintf(`let lib = import %q; parse = import %q { inherit lib; }; in parse.kernels.macos.families`, libNix, parseNix)
	expr, err := parser.ParseString(src)
	if err != nil { t.Fatal(err) }
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil { t.Fatal(err) }
	fv, err := ev.ForceValue(val)
	if err != nil { t.Fatal(err) }
	sym := ev.Intern.Intern("darwin")
	if fv.Attrs.Binds[sym] == nil {
		t.Fatalf("got %s", fv.DisplayString(ev, ev.Intern))
	}
}
