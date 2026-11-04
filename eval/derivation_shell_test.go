package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestDerivationExposesEnvAttrsAtTopLevel(t *testing.T) {
	src := `let d = builtins.derivation {
	  name = "foo";
	  system = "x86_64-linux";
	  builder = "/bin/bash";
	  shell = "/bin/bash";
	}; in {
	  hasShell = d ? shell;
	  shell = d.shell or "missing";
	  envShell = d.env.shell or "missing";
	}`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	hasShell := fv.Attrs.Binds[ev.Intern.Intern("hasShell")]
	shell := fv.Attrs.Binds[ev.Intern.Intern("shell")]
	envShell := fv.Attrs.Binds[ev.Intern.Intern("envShell")]
	forceBind := func(label string, b *eval.AttrBinding) *eval.Value {
		t.Helper()
		var raw *eval.Value
		if b.Thunk != nil {
			raw = eval.Deferred(b.Thunk)
		} else {
			raw = b.Value
		}
		val, err := ev.ForceValue(raw)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		return val
	}
	hs := forceBind("hasShell", hasShell)
	sh := forceBind("shell", shell)
	es := forceBind("envShell", envShell)
	t.Logf("hasShell=%v shell=%q envShell=%q", hs.Bool, sh.String, es.String)
	if sh.String == "missing" || !hs.Bool {
		t.Fatal("derivation should expose shell at top level like Nix")
	}
}
