package eval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func testModRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		p := filepath.Dir(wd)
		if p == wd {
			t.Fatal("no go.mod")
		}
		wd = p
	}
}

func TestImportNixpkgsEvalExprOnly(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `import "` + filepath.Join(nixpkgs, "default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	_, err = ev.evalExpr(expr, env)
	if err != nil {
		t.Fatalf("evalExpr: %v", err)
	}
}

func TestImportSelectStdenvTypeEvalExprOnly(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `let pkgs = import "` + filepath.Join(nixpkgs, "default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in builtins.typeOf pkgs`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	_, err = ev.evalExpr(expr, env)
	if err != nil {
		t.Fatalf("evalExpr: %v", err)
	}
}

func TestImportNixpkgsForceValueFails(t *testing.T) {
	root := testModRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	src := `import "` + filepath.Join(nixpkgs, "default.nix") + `" { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }`
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	v, err := ev.evalExpr(expr, env)
	if err != nil {
		t.Fatalf("evalExpr: %v", err)
	}
	fv, err := ev.forceValue(v)
	if err != nil {
		t.Fatalf("forceValue: %v (eval head kind was %v)", err, v.Kind)
	}
	if fv.Kind != VKAttrs {
		t.Fatalf("expected attrs WHNF, got %v", fv.Kind)
	}
}
