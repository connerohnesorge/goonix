package eval_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestFlakeRegistryResolvesLikeNix(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil && nix224Binary() == "" {
		t.Skip("no nix-instantiate")
	}
	if nix224Binary() == "" {
		oracle.RequireNix224(t)
	}

	root := t.TempDir()
	flakeA := filepath.Join(root, "flakeA")
	flakeB := filepath.Join(root, "flakeB")
	for _, dir := range []string{flakeA, flakeB} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(flakeA, "flake.nix"), []byte("{ outputs = { self }: { testValue = \"A\"; }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flakeA, "body"), []byte("A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flakeB, "flake.nix"), []byte("{ outputs = { self }: { testValue = \"B\"; }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flakeB, "body"), []byte("B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	userDir := filepath.Join(root, "user")
	sysDir := filepath.Join(root, "sys")
	if err := os.MkdirAll(filepath.Join(userDir, "nix"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sysDir, 0o755); err != nil {
		t.Fatal(err)
	}
	global := filepath.Join(root, "global.json")
	writeRegistry(t, filepath.Join(userDir, "nix", "registry.json"), []regFlake{
		{From: map[string]string{"type": "indirect", "id": "foo"}, To: map[string]string{"type": "path", "path": flakeA}},
		{From: map[string]string{"type": "indirect", "id": "chain"}, To: map[string]string{"type": "indirect", "id": "bar"}},
	})
	writeRegistry(t, filepath.Join(sysDir, "registry.json"), []regFlake{
		{From: map[string]string{"type": "indirect", "id": "foo"}, To: map[string]string{"type": "path", "path": flakeB}},
		{From: map[string]string{"type": "indirect", "id": "bar", "ref": "main"}, To: map[string]string{"type": "path", "path": flakeA}},
		{From: map[string]string{"type": "indirect", "id": "bar"}, To: map[string]string{"type": "path", "path": flakeB}},
	})
	writeRegistry(t, global, []regFlake{
		{From: map[string]string{"type": "indirect", "id": "foo"}, To: map[string]string{"type": "path", "path": flakeB}},
		{From: map[string]string{"type": "indirect", "id": "onlyglobal"}, To: map[string]string{"type": "path", "path": flakeB}},
	})

	t.Setenv("HOME", userDir)
	t.Setenv("XDG_CONFIG_HOME", userDir)
	t.Setenv("NIX_CONF_DIR", sysDir)
	t.Setenv("NIX_CONFIG", "experimental-features = nix-command flakes\nflake-registry = "+global+"\n")

	cases := []string{
		`let t = builtins.fetchTree "flake:foo"; in "${t.outPath} ${t.narHash}"`,
		`let t = builtins.fetchTree { type = "indirect"; id = "foo"; }; in "${t.outPath} ${t.narHash}"`,
		`let t = builtins.getFlake "foo"; in "${t.outPath} ${t.narHash}"`,
		`let t = builtins.fetchTree "flake:onlyglobal"; in "${t.outPath} ${t.narHash}"`,
		`let t = builtins.fetchTree "flake:chain/main"; in "${t.outPath} ${t.narHash}"`,
		`let t = builtins.fetchTree "flake:chain"; in "${t.outPath} ${t.narHash}"`,
	}
	for _, expr := range cases {
		want := nixEvalRegistry(t, expr)
		got := evalPkgsExprString(t, expr)
		if got != want {
			t.Fatalf("%s\ngo=%s\nnix=%s", expr, got, want)
		}
	}
	// User registry wins over system and global, so foo is flake A.
	directA := evalPkgsExprString(t, `let t = builtins.fetchTree { type = "path"; path = "`+flakeA+`"; }; in "${t.outPath} ${t.narHash}"`)
	foo := evalPkgsExprString(t, `let t = builtins.fetchTree "flake:foo"; in "${t.outPath} ${t.narHash}"`)
	if foo != directA {
		t.Fatalf("user registry did not win\nfoo=%s\nA=%s", foo, directA)
	}
	chainMain := evalPkgsExprString(t, `let t = builtins.fetchTree "flake:chain/main"; in "${t.outPath} ${t.narHash}"`)
	if chainMain != directA {
		t.Fatalf("ref override\nchain/main=%s\nA=%s", chainMain, directA)
	}
	directB := evalPkgsExprString(t, `let t = builtins.fetchTree { type = "path"; path = "`+flakeB+`"; }; in "${t.outPath} ${t.narHash}"`)
	chain := evalPkgsExprString(t, `let t = builtins.fetchTree "flake:chain"; in "${t.outPath} ${t.narHash}"`)
	if chain != directB {
		t.Fatalf("chain without ref\nchain=%s\nB=%s", chain, directB)
	}

	_, nerr := nixEvalRegistryErr(t, `builtins.fetchTree { type = "indirect"; id = "not-a-real-flake-id-goonix"; }`)
	_, gerr := evalPkgsExprStringAllowError(t, `builtins.fetchTree { type = "indirect"; id = "not-a-real-flake-id-goonix"; }`)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), "cannot find flake 'flake:not-a-real-flake-id-goonix'") || !strings.Contains(gerr.Error(), "cannot find flake 'flake:not-a-real-flake-id-goonix'") {
		t.Fatalf("missing id\nnix=%v\ngo=%v", nerr, gerr)
	}
}

type regFlake struct {
	From  map[string]string `json:"from"`
	To    map[string]string `json:"to"`
	Exact bool              `json:"exact,omitempty"`
}

func writeRegistry(t *testing.T, path string, flakes []regFlake) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"version": 2, "flakes": flakes})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func nix224Binary() string {
	// The local Nix 2.24.9 tree is bind-mounted over /nix/store by the
	// wrapper. On the host the binary lives under /tmp/nix224/store.
	p := "/tmp/nix224/store/2nhrwv91g6ycpyxvhmvc0xs8p92wp4bk-nix-2.24.9/bin/nix-instantiate"
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func nixEvalRegistry(t *testing.T, expr string) string {
	t.Helper()
	out, err := nixEvalRegistryErr(t, expr)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func nixEvalRegistryErr(t *testing.T, expr string) (string, error) {
	t.Helper()
	args := []string{"--eval", "--json", "-E", expr}
	var cmd *exec.Cmd
	if bin := nix224Binary(); bin != "" {
		// Run the 2.24.9 binary directly. The PATH wrapper resets HOME and
		// NIX_CONF_DIR, which hides the user and system registries.
		script := `mount --bind /tmp/nix224/store /nix/store && export LD_PRELOAD=/tmp/nochown.so && exec "$0" "$@"`
		cmd = exec.Command("unshare", append([]string{"-rm", "--propagation", "private", "bash", "-c", script, bin}, args...)...)
	} else {
		cmd = exec.Command("nix-instantiate", args...)
	}
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &registryEvalError{msg: strings.TrimSpace(string(out))}
	}
	var s string
	if err := json.Unmarshal(out, &s); err != nil {
		return strings.TrimSpace(string(out)), nil
	}
	return s, nil
}

type registryEvalError struct{ msg string }

func (e *registryEvalError) Error() string { return e.msg }
