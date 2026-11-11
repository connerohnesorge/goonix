package local

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGCKeepsRootsAndReferences(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	kept, err := st.AddToStoreText("kept", "keep-me\n")
	if err != nil {
		t.Fatal(err)
	}
	referrer, err := st.AddToStoreText("referrer", "see "+kept+"\n")
	if err != nil {
		t.Fatal(err)
	}
	junk, err := st.AddToStoreText("junk", "delete-me\n")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := st.QueryReferences(referrer)
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0] != kept {
		t.Fatalf("referrer refs=%v want %s", refs, kept)
	}

	rootLink := filepath.Join(t.TempDir(), "result")
	if err := st.AddPermRoot(referrer, rootLink); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(rootLink)
	if err != nil || target != referrer {
		t.Fatalf("perm root -> %q err %v", target, err)
	}
	autoName, err := nix32SHA1(rootLink)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(st.PhysicalGCRootsDir(), "auto", autoName)); err != nil {
		t.Fatalf("indirect root: %v", err)
	}

	deleted, err := st.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != junk {
		t.Fatalf("deleted %v, want only %s", deleted, junk)
	}
	for _, p := range []string{kept, referrer} {
		ok, err := st.IsValidPath(p)
		if err != nil || !ok {
			t.Fatalf("%s valid=%v err=%v", p, ok, err)
		}
		if _, err := os.Lstat(PhysicalPath(st.Root, p)); err != nil {
			t.Fatalf("physical %s: %v", p, err)
		}
	}
	if _, err := os.Lstat(PhysicalPath(st.Root, junk)); !os.IsNotExist(err) {
		t.Fatalf("junk still on disk: %v", err)
	}

	if err := os.Remove(rootLink); err != nil {
		t.Fatal(err)
	}
	deleted, err = st.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(st.PhysicalGCRootsDir(), "auto", autoName)); !os.IsNotExist(err) {
		t.Fatalf("stale auto root still present: %v", err)
	}
	got := map[string]bool{}
	for _, p := range deleted {
		got[p] = true
	}
	if !got[kept] || !got[referrer] {
		t.Fatalf("after dropping the root, deleted %v", deleted)
	}
}

func TestDirectGCRootPinsPath(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	keep, err := st.AddToStoreText("direct", "d\n")
	if err != nil {
		t.Fatal(err)
	}
	drop, err := st.AddToStoreText("drop", "x\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGCRoot("pinned", keep); err != nil {
		t.Fatal(err)
	}
	deleted, err := st.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != drop {
		t.Fatalf("deleted %v", deleted)
	}
	ok, err := st.IsValidPath(keep)
	if err != nil || !ok {
		t.Fatalf("pinned path valid=%v err=%v", ok, err)
	}
}

func TestAutoRootHashMatchesNixHash(t *testing.T) {
	if _, err := exec.LookPath("nix-hash"); err != nil {
		t.Skip("nix-hash not installed")
	}
	path := "/var/example/goonix-gc-root"
	f := filepath.Join(t.TempDir(), "bytes")
	if err := os.WriteFile(f, []byte(path), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("nix-hash", "--type", "sha1", "--base32", "--flat", f).Output()
	if err != nil {
		t.Fatal(err)
	}
	got, err := nix32SHA1(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(out))
	if got != want {
		t.Fatalf("auto root hash go=%s nix-hash=%s", got, want)
	}
}
