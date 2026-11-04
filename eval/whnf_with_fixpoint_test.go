package eval

import (
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestTypeOfFixpointAttrSetDoesNotForceAllBindings(t *testing.T) {
	src := `
let
  self = rec {
    stdenv = { cc = { isGNU = true; }; };
    pkgset = with self; rec {
      gccStdenv = if stdenv.cc.isGNU then stdenv else null;
      unused = builtins.throw "must not force unused";
    };
  };
in builtins.typeOf self.pkgset
`
	ev := NewEvaluator()
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.evalExpr(expr, env)
	if err != nil {
		t.Fatalf("evalExpr: %v", err)
	}
	fv, err := ev.forceValue(v)
	if err != nil {
		t.Fatalf("forceValue: %v", err)
	}
	if fv.Kind != VKString || fv.String != "set" {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

func TestTryEvalDoesNotDeepForce(t *testing.T) {
	src := `
let
  s = { a = 1; b = builtins.throw "deep"; };
  r = builtins.tryEval s;
in r.success
`
	ev := NewEvaluator()
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
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
		t.Fatal(err)
	}
	if fv.Kind != VKBool || !fv.Bool {
		t.Fatalf("tryEval should succeed without deep forcing b, got %v", fv)
	}
}
