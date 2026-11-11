package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestCurlDarwinAtPackage(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in { curlIsDarwin = pkgs.curl.stdenv.hostPlatform.isDarwin; pkgsIsDarwin = pkgs.stdenv.hostPlatform.isDarwin; }`
	want := lay.EvalStdinRaw(t, `builtins.toJSON (`+src+`)`)
	expr, _ := parser.ParseString(`builtins.toJSON (` + src + `)`)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestCurlPnameOnly(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; }; in pkgs.curl.pname`
	expr, _ := parser.ParseString(src)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	_, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
