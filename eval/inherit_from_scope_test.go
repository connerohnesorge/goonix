package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// inherit (lib) must resolve the outer lambda argument, not with-shadowed pkgs.lib.
func TestInheritFromUsesOuterLibNotWithPkgs(t *testing.T) {
	src := `
let
  lib = { recurseIntoAttrs = x: x // { tag = "fn"; }; };
  pkgs = { lib = { recurseIntoAttrs = x: x // { tag = "wrong"; }; }; };
in (with pkgs; {
  inherit (lib) recurseIntoAttrs;
}).recurseIntoAttrs { a = 1; }
`
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
	if fv.Kind != eval.VKAttrs {
		t.Fatalf("got %s", fv.DisplayString(ev, ev.Intern))
	}
	tag := ev.Intern.Intern("tag")
	b, ok := fv.Attrs.Binds[tag]
	if !ok {
		t.Fatalf("got %s", fv.DisplayString(ev, ev.Intern))
	}
	var tv *eval.Value
	if b.Thunk != nil {
		tv, err = ev.ForceValue(eval.Deferred(b.Thunk))
	} else {
		tv = b.Value
	}
	if err != nil {
		t.Fatal(err)
	}
	tv, err = ev.ForceValue(tv)
	if err != nil {
		t.Fatal(err)
	}
	if tv.Kind != eval.VKString || tv.String != "fn" {
		t.Fatalf("got %s", fv.DisplayString(ev, ev.Intern))
	}
}
