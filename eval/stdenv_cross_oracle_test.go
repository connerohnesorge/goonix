package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/internal/testutil"
	"github.com/connerohnesorge/goonix/parser"
)

// Oracle: subexpr types from pkgs/stdenv/cross/default.nix line 23 (`lib.init bootStages`).
func TestStdenvCrossLine23Subexprs(t *testing.T) {
	nixpkgs := testutil.NixpkgsPath(t)
	libPath := filepath.Join(nixpkgs, "lib")
	stdenvPath := filepath.Join(nixpkgs, "pkgs/stdenv")

	src := `
let
  lib = import "` + libPath + `";
  localSystem = lib.systems.elaborate { system = "x86_64-linux"; };
  overlays = [ ];
  bootStages = import "` + stdenvPath + `" {
    inherit lib localSystem overlays;
    crossSystem = localSystem;
    crossOverlays = [ ];
    config = { contentAddressedByDefault = false; };
  };
in {
  bootStagesType = builtins.typeOf bootStages;
  initType = builtins.typeOf lib.init;
  initBootStagesType = builtins.typeOf (lib.init bootStages);
  initLen = builtins.length (lib.init bootStages);
}
`
	nixFile := filepath.Join(t.TempDir(), "cross-subexpr.nix")
	if err := os.WriteFile(nixFile, []byte("{ }:\n"+strings.TrimSpace(src)), 0o644); err != nil {
		t.Fatal(err)
	}
	lay := oracle.MustLayout(t)
	nixV := lay.EvalImportJSON(t, nixFile)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
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
		t.Fatalf("expected attrset result, got %v", fv.Kind)
	}
	readType := func(name string) string {
		sym := ev.Intern.Intern(name)
		b := fv.Attrs.Binds[sym]
		if b == nil {
			t.Fatalf("missing %q", name)
		}
		var v *eval.Value
		if b.Thunk != nil {
			v, err = ev.ForceValue(eval.Deferred(b.Thunk))
		} else {
			v = b.Value
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fvs, err := ev.ForceValue(v)
		if err != nil {
			t.Fatalf("%s force: %v", name, err)
		}
		if fvs.Kind != eval.VKString {
			t.Fatalf("%s: got kind %v", name, fvs.Kind)
		}
		return fvs.String
	}
	readInt := func(name string) int {
		sym := ev.Intern.Intern(name)
		b := fv.Attrs.Binds[sym]
		if b == nil {
			t.Fatalf("missing %q", name)
		}
		var v *eval.Value
		if b.Thunk != nil {
			v, err = ev.ForceValue(eval.Deferred(b.Thunk))
		} else {
			v = b.Value
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fvs, err := ev.ForceValue(v)
		if err != nil {
			t.Fatalf("%s force: %v", name, err)
		}
		if fvs.Kind != eval.VKInt {
			t.Fatalf("%s: got kind %v", name, fvs.Kind)
		}
		return int(fvs.Int)
	}
	for _, k := range []string{"bootStagesType", "initType", "initBootStagesType"} {
		want, _ := nixV[k].(string)
		got := readType(k)
		if got != want {
			t.Fatalf("%s: goonix=%q nix=%q", k, got, want)
		}
	}
	wantLen, _ := nixV["initLen"].(float64)
	if gotLen := readInt("initLen"); float64(gotLen) != wantLen {
		t.Fatalf("initLen: goonix=%d nix=%v", gotLen, wantLen)
	}
}
