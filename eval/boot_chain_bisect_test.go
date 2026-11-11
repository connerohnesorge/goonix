package eval_test

import (
	"fmt"
	"os/exec"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestBootChainBisectMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_chain_bisect_probe.nix")
	abs, _ := filepath.Abs(path)

	for depth := 0; depth < 8; depth++ {
		depth := depth
		t.Run(fmt.Sprintf("depth%d", depth), func(t *testing.T) {
			nixV := evalOracleNixPick(t, abs, depth)
			goV, err := evalGoonixImportArgsAllowErr(t, root, abs, fmt.Sprintf("{ pick = %d; }", depth))
			if err != nil {
				t.Fatalf("go depth %d: %v", depth, err)
			}
			if !jsonEqual(t, nixV, goV) {
				t.Fatalf("depth %d differs:\nnix=%#v\ngo=%#v", depth, nixV, goV)
			}
		})
	}
}

func evalOracleNixPick(t *testing.T, absPath string, pick int) map[string]any {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	expr := fmt.Sprintf(`import "%s" { pick = %d; }`, absPath, pick)
	out, err := exec.Command("nix-instantiate", "--eval", "--strict", "--json", "-E", expr).Output()
	if err != nil {
		t.Fatalf("nix pick %d: %v", pick, err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func evalGoonixImportArgsAllowErr(t *testing.T, root, absPath, args string) (map[string]any, error) {
	t.Helper()
	src := `import "` + absPath + `" ` + args
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + filepath.Join(root, "testdata", "nixpkgs"))
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		return nil, err
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		return nil, err
	}
	return valueToJSONMapAllowErr(t, ev, val)
}
