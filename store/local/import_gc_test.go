package local

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func narOf(t *testing.T, dir string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	if err := DumpPathToWriter(&buf, dir); err != nil {
		t.Fatal(err)
	}
	h, _, err := HashNARStream(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), h
}

func TestImportNARStreamsAndChecksHash(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "f"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	nar, hash := narOf(t, src)
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	t.Cleanup(func() { _ = RemoveStoreTree(st.Root) })

	bad := "/nix/store/00000000000000000000000000000000-bad"
	if err := st.ImportNAR(bad, bytes.NewReader(nar), strings.Repeat("0", len(hash))); err == nil {
		t.Fatal("wrong hash accepted")
	}
	if _, err := os.Lstat(PhysicalPath(st.Root, bad)); err == nil {
		t.Fatal("a NAR with the wrong hash was left in the store")
	}
	if ok, _ := st.IsValidPath(bad); ok {
		t.Fatal("a NAR with the wrong hash was registered")
	}

	good := "/nix/store/11111111111111111111111111111111-good"
	if err := st.ImportNAR(good, bytes.NewReader(nar), hash); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(PhysicalPath(st.Root, good), "f"))
	if err != nil || string(b) != "hi" {
		t.Fatalf("imported file = %q, %v", b, err)
	}
	meta, err := st.QueryPathMeta(good)
	if err != nil || meta.NarSize != int64(len(nar)) {
		t.Fatalf("narSize = %+v, %v; want %d", meta, err, len(nar))
	}
	entries, _ := os.ReadDir(filepath.Dir(PhysicalPath(st.Root, good)))
	for _, e := range entries {
		if e.Name()[0] == '.' {
			t.Fatalf("temporary %s left behind", e.Name())
		}
	}
}

// Store directories are read-only; garbage collection must still delete them.
func TestCollectGarbageDeletesReadOnlyDirectory(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	nar, hash := narOf(t, src)
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	p := "/nix/store/22222222222222222222222222222222-dir"
	if err := st.ImportNAR(p, bytes.NewReader(nar), hash); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(PhysicalPath(st.Root, p), "sub")); err != nil || fi.Mode().Perm()&0o200 != 0 {
		t.Fatalf("expected a read-only store directory, got %v, %v", fi.Mode(), err)
	}
	deleted, err := st.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != p {
		t.Fatalf("deleted = %v", deleted)
	}
	if _, err := os.Lstat(PhysicalPath(st.Root, p)); err == nil {
		t.Fatal("store directory still on disk")
	}
}
