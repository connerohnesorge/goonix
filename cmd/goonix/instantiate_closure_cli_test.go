package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func TestInstantiateAndBuildWriteDependencyDrvs(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	root := t.TempDir()
	nixFile := filepath.Join(root, "t.nix")
	src := `let
  dep = derivation {
    name = "cli-dep";
    system = "x86_64-linux";
    builder = "/bin/sh";
    args = [ "-c" "printf dep > $out" ];
  };
in derivation {
  name = "cli-top";
  system = "x86_64-linux";
  builder = "/bin/sh";
  args = [ "-c" "printf top > $out" ];
  inherit dep;
}
`
	if err := os.WriteFile(nixFile, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := buildGoonix(t, root)

	instRoot := filepath.Join(root, "inst-root")
	if err := os.MkdirAll(instRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "instantiate", nixFile)
	cmd.Env = append(os.Environ(), "GOONIX_CHROOT_PHYS="+instRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goonix instantiate: %v\n%s", err, out)
	}
	top := strings.TrimSpace(string(out))
	if !strings.HasPrefix(top, "/nix/store/") || !strings.HasSuffix(top, "-cli-top.drv") {
		t.Fatalf("instantiate printed %q", top)
	}
	assertDrvClosure(t, filepath.Join(instRoot, "store"), store.Config{})

	buildStore := filepath.Join(root, "build-store")
	args := []string{"build", "--store", buildStore}
	if err := exec.Command("unshare", "-m", "true").Run(); err != nil {
		args = append(args, "--allow-relocated-store")
	}
	args = append(args, nixFile)
	cmd = exec.Command(bin, args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goonix build: %v\n%s", err, out)
	}
	assertDrvClosure(t, filepath.Join(buildStore, "store"), store.Config{})
}

func assertDrvClosure(t *testing.T, dir string, cfg store.Config) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.drv"))
	if err != nil {
		t.Fatal(err)
	}
	var dep, top bool
	for _, path := range matches {
		base := filepath.Base(path)
		if strings.Contains(base, "-cli-dep.drv") {
			dep = true
		}
		if strings.Contains(base, "-cli-top.drv") {
			top = true
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := store.DrvPathFromATermBytes(cfg, body)
		if err != nil {
			t.Fatal(err)
		}
		want := cfg.LogicalStoreDir() + "/" + base
		if got != want {
			t.Fatalf("%s hashes to %s", path, got)
		}
	}
	if !dep || !top {
		t.Fatalf("drv files in %s: %v", dir, matches)
	}
}
