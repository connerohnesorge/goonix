package eval

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestBootstrapXgccStrictEnvOmitsNullDrvAttrs(t *testing.T) {
	lay := oracle.MustLayout(t)
	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	helper := filepath.Join(lay.Root, "eval", "testdata", "xgcc_drvattrs_for_strict.nix")
	ev.SetCurFile(helper)
	expr, err := parser.ParseFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	attrsV, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	attrsV, err = ev.ForceValue(attrsV)
	if err != nil {
		t.Fatal(err)
	}
	intern := ev.Intern
	if b := attrsV.Attrs.Binds[intern.Intern("__ignoreNulls")]; b != nil {
		iv, err := ev.attrsBindingForce(b)
		if err != nil {
			t.Fatal(err)
		}
		iv, err = ev.forceValue(iv)
		if err != nil {
			t.Fatal(err)
		}
		if iv.Kind != VKBool || !iv.Bool {
			t.Fatalf("__ignoreNulls kind=%v bool=%v", iv.Kind, iv.Bool)
		}
	} else {
		t.Fatal("drvAttrs missing __ignoreNulls")
	}
	if b := attrsV.Attrs.Binds[intern.Intern("libcCross")]; b != nil {
		lv, err := ev.attrsBindingForce(b)
		if err != nil {
			t.Fatal(err)
		}
		lv, err = ev.forceValue(lv)
		if err != nil {
			t.Fatal(err)
		}
		if lv.Kind != VKNull {
			t.Fatalf("libcCross kind=%v string=%q", lv.Kind, lv.String)
		}
	} else {
		t.Fatal("drvAttrs missing libcCross key")
	}
	drv, err := ev.AttrsToDerivation(attrsV)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := drv.Env["libcCross"]; ok {
		t.Fatalf("derivationStrict env has libcCross=%q", drv.Env["libcCross"])
	}
	if _, ok := drv.Env["targetConfig"]; ok {
		t.Fatalf("derivationStrict env has targetConfig=%q", drv.Env["targetConfig"])
	}
}
