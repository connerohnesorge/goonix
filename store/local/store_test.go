// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gonixnar "github.com/nix-community/go-nix/pkg/nar"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestNarHashMatchesGoNixDump(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(sub, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested", "a"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, size, err := local.NarHashAndSize(sub)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := gonixnar.DumpPath(&buf, sub); err != nil {
		t.Fatal(err)
	}
	want, _, err := local.HashNARStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("hash goonix=%s go-nix=%s", got, want)
	}
	if size != int64(buf.Len()) {
		t.Fatalf("size=%d buf=%d", size, buf.Len())
	}
}

func TestAddToStoreTextAndReferences(t *testing.T) {
	root := t.TempDir()
	st, err := local.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	dep, err := st.AddToStoreText("dep-doc", "no refs here")
	if err != nil {
		t.Fatal(err)
	}
	body := "uses " + dep
	path, err := st.AddToStoreText("main-doc", body)
	if err != nil {
		t.Fatal(err)
	}
	ok, err := st.IsValidPath(path)
	if err != nil || !ok {
		t.Fatalf("valid=%v err=%v", ok, err)
	}
	refs, err := st.QueryReferences(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != dep {
		t.Fatalf("refs=%v want [%s]", refs, dep)
	}
}

func TestNarHashMatchesNixStoreDump(t *testing.T) {
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("nix-store not on PATH")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "sample")
	if err := os.WriteFile(p, []byte("xyzzy"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("nix-store", "--dump", p).Output()
	if err != nil {
		t.Fatalf("nix-store --dump: %v", err)
	}
	want, _, err := local.HashNARStream(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	got, _, err := local.FlatNarHashAndSize(p)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("flat hash go=%s nix=%s", got, want)
	}
}

func TestGCRootStub(t *testing.T) {
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	p, err := st.AddToStoreText("gc-test", "x")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGCRoot("test-root", p); err != nil {
		t.Fatal(err)
	}
	got, ok, err := st.GCRootPath("test-root")
	if err != nil || !ok || got != p {
		t.Fatalf("root=%q ok=%v err=%v", got, ok, err)
	}
}

var _ = strings.Builder{}
