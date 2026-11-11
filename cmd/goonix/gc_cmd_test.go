package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/store/local"
)

func TestGoonixGCRespectsRoots(t *testing.T) {
	root := t.TempDir()
	st, err := local.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := st.AddToStoreText("keep", "keep\n")
	if err != nil {
		t.Fatal(err)
	}
	drop, err := st.AddToStoreText("drop", "drop\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGCRoot("keep-root", keep); err != nil {
		t.Fatal(err)
	}
	st.Close()

	bin := filepath.Join(t.TempDir(), "goonix")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = moduleDir(t)
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "gc", "--store", root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gc: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), drop) {
		t.Fatalf("gc output %q does not delete %s", out, drop)
	}
	if strings.Contains(string(out), keep) {
		t.Fatalf("gc deleted a rooted path: %s", out)
	}
	st, err = local.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ok, err := st.IsValidPath(keep)
	if err != nil || !ok {
		t.Fatalf("keep valid=%v err=%v", ok, err)
	}
	ok, err = st.IsValidPath(drop)
	if err != nil || ok {
		t.Fatalf("drop valid=%v err=%v", ok, err)
	}
}

func moduleDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return wd
}
