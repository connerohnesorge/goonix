package eval_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBootStagesTakeBisectMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_stages_take_bisect_probe.nix")
	abs, _ := filepath.Abs(path)
	const maxK = 11
	for k := 1; k <= maxK; k++ {
		k := k
		t.Run(fmt.Sprintf("k%d", k), func(t *testing.T) {
			nixV := evalOracleNixPickJSON(t, abs, k)
			goV, err := evalGoonixImportArgsAllowErr(t, root, abs, fmt.Sprintf("{ pick = %d; }", k))
			if err != nil {
				t.Fatalf("go k=%d: %v", k, err)
			}
			if !jsonEqual(t, nixV, goV) {
				t.Fatalf("k=%d differs:\nnix=%#v\ngo=%#v", k, nixV, goV)
			}
		})
	}
}

// Nix strict mode rejects missing-attribute errors before tryEval; oracle uses non-strict JSON like import_head probes.
func evalOracleNixPickJSON(t *testing.T, absPath string, pick int) map[string]any {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	expr := fmt.Sprintf(`import "%s" { pick = %d; }`, absPath, pick)
	out, err := exec.Command("nix-instantiate", "--eval", "--strict", "--json", "-E", expr).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("nix pick %d: %s\n%s", pick, err, ee.Stderr)
		}
		t.Fatalf("nix pick %d: %v", pick, err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return v
}

func TestBootStagesTakeFirstDivergence(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_stages_take_bisect_probe.nix")
	abs, _ := filepath.Abs(path)
	const maxK = 11
	for k := 1; k <= maxK; k++ {
		nixV := evalOracleNixPickJSON(t, abs, k)
		goV, err := evalGoonixImportArgsAllowErr(t, root, abs, fmt.Sprintf("{ pick = %d; }", k))
		if err != nil {
			t.Fatalf("first divergence k=%d: go eval err %v (nix=%#v)", k, err, nixV)
		}
		if !jsonEqual(t, nixV, goV) {
			t.Fatalf("first divergence k=%d:\nnix=%#v\ngo=%#v", k, nixV, goV)
		}
	}
	t.Log("all k=1..11 match nix")
}
