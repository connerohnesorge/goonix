package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestLetInheritDarwinWithMapAttrsSetTypes(t *testing.T) {
	src := `
let
  execFormats = { macho = { }; elf = { }; };
  kernelFamilies = { bsd = { }; darwin = { }; };
  inherit (execFormats) macho elf;
  inherit (kernelFamilies) bsd darwin;
  setTypes = type: builtins.mapAttrs (name: value: value);
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
