package local

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

// A NAR round-trips through restore and dump byte for byte, including an
// executable file, a nested directory, a relative link, a link to another
// link, and an absolute link whose target does not exist on this host.
func TestNARRestoreRoundTrip(t *testing.T) {
	src := t.TempDir()
	tree := filepath.Join(src, "tree")
	for _, d := range []string{"bin", "share/doc"} {
		if err := os.MkdirAll(filepath.Join(tree, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tree, "bin", "tool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "share", "doc", "README"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"share/tool":   "../bin/tool",
		"share/again":  "tool",
		"share/docdir": "doc",
		"dangling":     "/nix/store/00000000000000000000000000000000-absent/bin/x",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(tree, name)); err != nil {
			t.Fatal(err)
		}
	}
	var want bytes.Buffer
	if err := store.DumpPath(&want, tree); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "restored")
	if err := unpackNAR(bytes.NewReader(want.Bytes()), out); err != nil {
		t.Fatal(err)
	}
	defer RemoveStoreTree(out)
	var got bytes.Buffer
	if err := store.DumpPath(&got, out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("restored tree dumps to a different NAR")
	}
	fi, err := os.Lstat(filepath.Join(out, "bin", "tool"))
	if err != nil {
		t.Fatal(err)
	}
	if !store.IsExecutable(filepath.Join(out, "bin", "tool"), fi) {
		t.Fatal("executable bit lost")
	}
	if fi.Mode().Perm()&0o200 != 0 {
		t.Fatal("restored file is writable")
	}
}

// A single executable file at the NAR root restores as a file.
func TestNARRestoreFlatExecutable(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	if err := os.WriteFile(f, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := store.DumpPath(&want, f); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "g")
	if err := unpackNAR(bytes.NewReader(want.Bytes()), out); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := store.DumpPath(&got, out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatal("flat executable did not round-trip")
	}
}
