package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestCurlPlatformProbeMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	nixpkgs := lay.Nixpkgs
	src := `let pkgs = import <nixpkgs> { localSystem = { system = "x86_64-linux"; }; config = { }; overlays = [ ]; };
in builtins.toJSON {
  isDarwin = pkgs.stdenv.hostPlatform.isDarwin;
  isLinux = pkgs.stdenv.hostPlatform.isLinux;
  kernel = pkgs.stdenv.hostPlatform.parsed.kernel.name;
  curlNames = map (p: p.name or (p.pname or "<?>")) pkgs.curl.buildInputs;
}`
	want := lay.EvalStdinRaw(t, src)

	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("platform probe mismatch\nnix=%s\ngo=%s", want, got)
	}
}
