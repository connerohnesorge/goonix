package eval_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestStdenvOverridesBootProbeMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "stdenv_overrides_boot_probe.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("stdenv overrides boot probe differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
	final, _ := goV["final"].(map[string]any)
	if final != nil && final["hasOverridesAttr"] != true {
		t.Fatalf("final stage stdenv missing overrides attr: %v", final)
	}
}

func TestDerivationMergePreservesOverridesAttr(t *testing.T) {
	src := `
let
  s =
    builtins.derivation {
      name = "stdenv-test";
      system = "x86_64-linux";
      builder = "/bin/sh";
      shell = "/bin/sh";
    }
    // {
      overrides = self: super: { bootGlibc = super.glibc or null; };
      hasCC = false;
      cc = null;
    };
in {
  hasOverrides = s ? overrides;
  isFunction = builtins.isFunction (s.overrides or null);
}
`
	nixV := evalOracleExpr(t, src)
	goV := evalGoonixExpr(t, src)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("derivation // overrides probe differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
	if nixV["hasOverrides"] != true {
		t.Fatalf("nix oracle expected hasOverrides true, got %v", nixV["hasOverrides"])
	}
}

func evalGoonixExpr(t *testing.T, src string) map[string]any {
	t.Helper()
	root := modRoot(t)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + filepath.Join(root, "testdata", "nixpkgs"))
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	return valueToJSONMap(t, ev, val)
}

func evalOracleExpr(t *testing.T, src string) map[string]any {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	out, err := exec.Command("nix-instantiate", "--eval", "--strict", "--json", "-E", src).Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
