package eval_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFlakeBuiltinsRequireFeatureLikeNix224(t *testing.T) {
	names := []string{"getFlake", "fetchTree", "parseFlakeRef", "flakeRefToString"}
	ev := eval.NewEvaluator()
	for _, name := range names {
		if _, ok := ev.Builtins[name]; ok {
			t.Fatalf("builtins.%s is registered with flakes disabled", name)
		}
	}
	for _, name := range names {
		if got := evalBool(t, ev, "builtins ? "+name); got {
			t.Fatalf("builtins ? %s is true", name)
		}
		namesJSON := evalString(t, ev, `builtins.concatStringsSep "," (builtins.attrNames builtins)`)
		if strings.Contains(","+namesJSON+",", ","+name+",") {
			t.Fatalf("attrNames contains %s: %s", name, namesJSON)
		}
		_, err := evalRaw(t, ev, "builtins."+name+" null")
		if err == nil || !strings.Contains(err.Error(), "attribute '"+name+"' missing") {
			t.Fatalf("builtins.%s: %v", name, err)
		}
	}

	on := eval.NewEvaluator().WithExperimental("flakes")
	for _, name := range names {
		if _, ok := on.Builtins[name]; !ok {
			t.Fatalf("builtins.%s missing after enabling flakes", name)
		}
	}
	got := evalString(t, on, `builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs")`)
	if got != "github:NixOS/nixpkgs" {
		t.Fatalf("got %q", got)
	}

	oracle.RequireNix224(t)
	for _, name := range names {
		nixHas := nixEval224(t, "builtins.toJSON (builtins ? "+name+")")
		if nixHas != "false" {
			t.Fatalf("nix 2.24.9 builtins ? %s = %s", name, nixHas)
		}
		nixNames := nixEval224(t, `builtins.concatStringsSep "," (builtins.attrNames builtins)`)
		if strings.Contains(","+nixNames+",", ","+name+",") {
			t.Fatalf("nix 2.24.9 attrNames contains %s", name)
		}
		_, nerr := nixEval224Err(t, "builtins."+name+" null")
		if nerr == nil || !strings.Contains(nerr.Error(), "attribute '"+name+"' missing") {
			t.Fatalf("nix 2.24.9 builtins.%s: %v", name, nerr)
		}
	}
}

func evalBool(t *testing.T, ev *eval.Evaluator, expr string) bool {
	t.Helper()
	v := mustEval(t, ev, expr)
	if v.Kind != eval.VKBool {
		t.Fatalf("%s: got %v", expr, v.Kind)
	}
	return v.Bool
}

func evalString(t *testing.T, ev *eval.Evaluator, expr string) string {
	t.Helper()
	v := mustEval(t, ev, expr)
	if v.Kind != eval.VKString {
		t.Fatalf("%s: got %v", expr, v.Kind)
	}
	return v.String
}

func evalRaw(t *testing.T, ev *eval.Evaluator, expr string) (*eval.Value, error) {
	t.Helper()
	parsed, err := parser.ParseString(expr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		return nil, err
	}
	return ev.ForceValue(val)
}

func mustEval(t *testing.T, ev *eval.Evaluator, expr string) *eval.Value {
	t.Helper()
	v, err := evalRaw(t, ev, expr)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func nixEval224(t *testing.T, expr string) string {
	t.Helper()
	s, err := nixEval224Err(t, expr)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func nixEval224Err(t *testing.T, expr string) (string, error) {
	t.Helper()
	cmd := exec.Command("nix-instantiate",
		"--option", "experimental-features", "nix-command",
		"--option", "substituters", "",
		"--eval", "--json", "-E", expr)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/tmp"}
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	s := strings.TrimSpace(string(out))
	var decoded string
	if jsonErr := json.Unmarshal([]byte(s), &decoded); jsonErr == nil {
		return decoded, nil
	}
	return s, nil
}
