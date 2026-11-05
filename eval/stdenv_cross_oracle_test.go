package eval_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// Line 23 of pkgs/stdenv/cross/default.nix: lib.init bootStages ++ [ … ]
func TestStdenvCrossLine23Subexprs(t *testing.T) {
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	libPath := filepath.Join(nixpkgs, "lib")
	stdenvPath := filepath.Join(nixpkgs, "pkgs/stdenv")
	crossPath := filepath.Join(nixpkgs, "pkgs/stdenv/cross/default.nix")

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
	got := map[string]string{
		"bootStagesType":     readType("bootStagesType"),
		"initType":           readType("initType"),
		"initBootStagesType": readType("initBootStagesType"),
	}
	want := map[string]string{
		"bootStagesType":     "list",
		"initType":           "lambda",
		"initBootStagesType": "list",
	}
	if nix, err := exec.LookPath("nix-instantiate"); err == nil {
		nixSrc := strings.ReplaceAll(src, "`", "")
		nixExpr := filepath.Join(t.TempDir(), "cross-subexpr.nix")
		if err := os.WriteFile(nixExpr, []byte(nixSrc), 0o644); err != nil {
			t.Fatal(err)
		}
		out, err := exec.Command(nix, "--eval", "-E", "import "+nixExpr).Output()
		if err == nil {
			t.Logf("nix oracle: %s", strings.TrimSpace(string(out)))
		}
		_ = nix
	}
	for k, w := range want {
		if got[k] != w {
			t.Fatalf("%s: goonix=%q want=%q (cross=%s)", k, got[k], w, crossPath)
		}
	}
	_ = crossPath
}
