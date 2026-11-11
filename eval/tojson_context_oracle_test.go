package eval_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestToJSONCarriesStringContextLikeNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	drvExpr := `let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in (derivation { name = "t"; system = "x86_64-linux"; builder = "/bin/sh"; j = builtins.toJSON { inherit d; }; }).drvPath`
	pathExpr := `let p = builtins.toFile "a" "hi"; in (derivation { name = "tp"; system = "x86_64-linux"; builder = "/bin/sh"; j = builtins.toJSON { inherit p; }; }).drvPath`
	for _, expr := range []string{drvExpr, pathExpr} {
		want := lay.EvalRaw(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("eval %s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("drvPath\nexpr=%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}

	toFileExpr := `let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; j = builtins.toJSON { inherit d; }; in builtins.toFile "j" j`
	_, goErr := goEvalString(t, toFileExpr)
	if goErr == nil {
		t.Fatal("toFile of toJSON derivation context must fail")
	}
	nixErr := nixEvalStderr(t, lay, toFileExpr)
	frag := "files created by builtins.toFile may not reference derivations, but j references !out!"
	if !strings.Contains(goErr.Error(), frag) || !strings.Contains(nixErr, frag) {
		t.Fatalf("toFile context error\ngo=%s\nnix=%s", goErr, nixErr)
	}
}

func TestToFileReturnsStringContextLikeNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	exprs := []string{
		`builtins.typeOf (builtins.toFile "n" "hi")`,
		`builtins.toJSON (builtins.getContext (builtins.toFile "n" "hi"))`,
	}
	for _, expr := range exprs {
		want := lay.EvalRaw(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("eval %s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
		if strings.Contains(expr, "typeOf") && got != "string" {
			t.Fatalf("typeOf %q", got)
		}
	}
}

func goEvalString(t *testing.T, expr string) (string, error) {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.ChrootIOEffects{PhysRoot: t.TempDir()})
	parsed, err := parser.ParseString(expr)
	if err != nil {
		return "", err
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		return "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	if fv.Kind != eval.VKString {
		t.Fatalf("expected string, got %v", fv.Kind)
	}
	return fv.String, nil
}

func nixEvalStderr(t *testing.T, lay oracle.Layout, expr string) string {
	t.Helper()
	args := append(lay.InstantiateArgs(), "--eval", "--json", "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = lay.HermeticEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected nix-instantiate to fail:\n%s", out)
	}
	return string(out)
}
