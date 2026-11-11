package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestLetInheritNestedAttr(t *testing.T) {
	src := `
let
  inherit ({ darwin = true; }) darwin;
in { f = { inherit darwin; }; }.f
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
	sym := ev.Intern.Intern("darwin")
	b := fv.Attrs.Binds[sym]
	if b == nil {
		t.Fatalf("expected darwin attr, got %s", fv.DisplayString(ev, ev.Intern))
	}
}

func TestLetInheritDarwinKernelStyle(t *testing.T) {
	src := `
let
  execFormats = { macho = { }; elf = { }; };
  kernelFamilies = { bsd = { }; darwin = { }; };
  inherit (execFormats) macho elf;
  inherit (kernelFamilies) bsd darwin;
  setTypes = t: attrs: attrs;
in (setTypes null {
  macos = {
    execFormat = macho;
    families = { inherit darwin; };
  };
}).macos.families
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
	sym := ev.Intern.Intern("darwin")
	if fv.Attrs.Binds[sym] == nil {
		t.Fatalf("got %s", fv.DisplayString(ev, ev.Intern))
	}
}

func TestLetAttrLookupViaParent(t *testing.T) {
	src := `
let
  parsed = { ok = true; inner = parsed.ok; };
in parsed.inner
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
	if fv.Kind != eval.VKBool || !fv.Bool {
		t.Fatalf("got %v", fv.DisplayString(ev, ev.Intern))
	}
}

