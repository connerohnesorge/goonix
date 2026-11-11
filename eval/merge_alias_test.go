package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestRecMergeAliasSelfRefLeftOperand(t *testing.T) {
	src := `
let kernelFamilies = { darwin = { }; };
in rec {
  kernels =
    let inherit (kernelFamilies) darwin;
    in { macos = { families = { inherit darwin; }; }; }
    // { darwin = kernels.macos; };
}
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
	// Force alias path
	sym := ev.Intern.Intern("kernels")
	kb := fv.Attrs.Binds[sym]
	if kb == nil || kb.Thunk == nil {
		t.Fatal("missing kernels")
	}
	kernels, err := ev.ForceValue(eval.Deferred(kb.Thunk))
	if err != nil {
		t.Fatal(err)
	}
	darwinSym := ev.Intern.Intern("darwin")
	db := kernels.Attrs.Binds[darwinSym]
	if db == nil {
		t.Fatal("missing darwin alias")
	}
	if db.Thunk != nil {
		_, err = ev.ForceValue(eval.Deferred(db.Thunk))
	} else {
		_, err = ev.ForceValue(db.Value)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecMergeAliasRealSetTypesAttrValues(t *testing.T) {
	src := `
let
  kernelFamilies = { darwin = { }; };
  types = {
    openKernel.check = x: builtins.isAttrs x.families;
  };
  setTypes = type: builtins.mapAttrs (name: value: assert type.check value; ({ inherit name; } // value));
in builtins.length (builtins.attrValues (
  (rec {
    kernels =
      let inherit (kernelFamilies) darwin;
      in setTypes types.openKernel {
        macos = { families = { inherit darwin; }; };
        linux = { families = { }; };
      }
      // { darwin = kernels.macos; };
  }).kernels
))
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
	ev.ForceValue(val)
}

func TestRecMergeAliasRealSetTypesMerge(t *testing.T) {
	src := `
let
  kernelFamilies = { darwin = { }; };
  types = {
    openKernel.check = x: builtins.isAttrs x.families;
  };
  setTypes = type: builtins.mapAttrs (name: value: assert type.check value; ({ inherit name; } // value));
in rec {
  kernels =
    let inherit (kernelFamilies) darwin;
    in setTypes types.openKernel {
      macos = { families = { inherit darwin; }; };
    }
    // { darwin = kernels.macos; };
}
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
	sym := ev.Intern.Intern("kernels")
	k, err := ev.ForceValue(eval.Deferred(fv.Attrs.Binds[sym].Thunk))
	if err != nil {
		t.Fatal(err)
	}
	darwinSym := ev.Intern.Intern("darwin")
	db := k.Attrs.Binds[darwinSym]
	if db.Thunk != nil {
		_, err = ev.ForceValue(eval.Deferred(db.Thunk))
	} else {
		_, err = ev.ForceValue(db.Value)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecMergeAliasWithMapAttrsSetTypes(t *testing.T) {
	src := `
let
  kernelFamilies = { darwin = { }; };
  setTypes = t: builtins.mapAttrs (n: v: v);
in rec {
  kernels =
    let inherit (kernelFamilies) darwin;
    in setTypes null {
      macos = { families = { inherit darwin; }; };
    }
    // { darwin = kernels.macos; };
}
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
	sym := ev.Intern.Intern("kernels")
	k, err := ev.ForceValue(eval.Deferred(fv.Attrs.Binds[sym].Thunk))
	if err != nil {
		t.Fatal(err)
	}
	darwinSym := ev.Intern.Intern("darwin")
	db := k.Attrs.Binds[darwinSym]
	if db.Thunk != nil {
		_, err = ev.ForceValue(eval.Deferred(db.Thunk))
	} else {
		_, err = ev.ForceValue(db.Value)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecMergeAliasAttrValues(t *testing.T) {
	src := `
let kernelFamilies = { darwin = { }; };
in builtins.length (builtins.attrValues (
  (let inherit (kernelFamilies) darwin;
   in rec {
     kernels =
       { macos = { families = { inherit darwin; }; }; }
       // { darwin = kernels.macos; };
   }
  ).kernels
))
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
	ev.ForceValue(val)
}
