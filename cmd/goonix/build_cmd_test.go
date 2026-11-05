package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/store/local"
)

func TestGoonixBuildEchoHiInTempStore(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	root := t.TempDir()
	storeRoot := filepath.Join(root, "goonix-store")
	nixFile := filepath.Join(root, "t.nix")
	src := `builtins.derivationStrict {
  name = "cli-echo";
  system = "x86_64-linux";
  builder = "/bin/sh";
  args = [ "-c" "echo hi > $out" ];
}`
	if err := os.WriteFile(nixFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin, err := exec.LookPath("goonix")
	if err != nil {
		// Build the CLI from this module when not installed.
		out := filepath.Join(root, "goonix-bin")
		build := exec.Command("go", "build", "-o", out, ".")
		build.Dir = filepath.Join(modRoot(t), "cmd", "goonix")
		if err := build.Run(); err != nil {
			t.Fatal(err)
		}
		bin = out
	}
	cmd := exec.Command(bin, "build", "--store", storeRoot, nixFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goonix build: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "out=/nix/store/") {
		t.Fatalf("unexpected output: %s", out)
	}
	st, err := local.OpenStore(storeRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 || kv[0] != "out" {
			continue
		}
		ok, err := st.IsValidPath(kv[1])
		if err != nil || !ok {
			t.Fatalf("output not valid in store: %q ok=%v err=%v", kv[1], ok, err)
		}
	}
}

func modRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}
