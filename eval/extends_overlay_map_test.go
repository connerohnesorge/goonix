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

func TestExtendsOverlayMapNamesDoesNotForceAllSuperDuringFixpoint(t *testing.T) {
	root := modRoot(t)
	repro := filepath.Join(root, "testdata", "oracle", "extends_overlay_map_super.nix")
	srcBytes, err := os.ReadFile(repro)
	if err != nil {
		t.Fatal(err)
	}
	src := string(srcBytes)

	if _, err := exec.LookPath("nix-instantiate"); err == nil {
		cmd := exec.Command("nix-instantiate", "--eval", "-E", "(builtins.tryEval ("+src+")).success")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("nix oracle: %v\n%s", err, out)
		}
		if !strings.Contains(strings.TrimSpace(string(out)), "true") {
			t.Fatalf("nix oracle expected success, got %q", out)
		}
	}

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
		t.Fatalf("force fixpoint: %v", err)
	}
	if fv == nil || fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("expected ok=1, got %v", fv)
	}
}

func TestLetSelfRecursiveBindsOneThunk(t *testing.T) {
	src := `
let
  f = self: { x = self.y; y = 1; };
  self = f self // { };
in self.x
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
		t.Fatalf("force: %v", err)
	}
	if fv == nil || fv.Kind != eval.VKInt || fv.Int != 1 {
		t.Fatalf("expected 1, got %v", fv)
	}
}
