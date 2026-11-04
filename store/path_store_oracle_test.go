package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func moduleRoot(t *testing.T) string {
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

func TestStorePathForFilesystemPathMatchesNixPath(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
	src := filepath.Join(moduleRoot(t), "testdata", "nixpkgs", "pkgs", "build-support", "setup-hooks", "update-autotools-gnu-config-scripts.sh")
	name := "update-autotools-gnu-config-scripts.sh"
	out, err := exec.Command("nix-instantiate", "--eval", "--strict",
		"-E", `builtins.path { path = `+src+`; name = "`+name+`"; }`,
	).Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	want := strings.Trim(strings.TrimSpace(string(out)), `"`)
	cfg := store.Config{}
	got, err := cfg.StorePathForFilesystemPath(src, name)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("store path go=%q nix=%q", got, want)
	}
}
