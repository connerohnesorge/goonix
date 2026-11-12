package local

import (
	"os"
	"path/filepath"
	"testing"
)

// A symlink output, or a symlink inside an output, refers to its target the
// way Nix's NAR scan sees it. The target need not exist on this host: the
// scan reads the link, it does not follow it.
func TestScanFindsReferencesInSymlinks(t *testing.T) {
	st, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	target, err := st.AddToStoreText("target", "hi\n")
	if err != nil {
		t.Fatal(err)
	}
	// Remove the target's physical copy so following the link would fail.
	if err := os.Remove(PhysicalPath(st.Root, target)); err != nil {
		t.Fatal(err)
	}

	t.Run("top-level link", func(t *testing.T) {
		out := "/nix/store/0000000000000000000000000000000a-link"
		if err := os.Symlink(target, PhysicalPath(st.Root, out)); err != nil {
			t.Fatal(err)
		}
		refs, err := st.ScanPathReferencesAmong(out, []string{out})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 1 || refs[0] != target {
			t.Fatalf("refs %v, want [%s]", refs, target)
		}
	})

	t.Run("link in directory", func(t *testing.T) {
		out := "/nix/store/0000000000000000000000000000000b-dir"
		phys := PhysicalPath(st.Root, out)
		if err := os.MkdirAll(filepath.Join(phys, "share"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(phys, "share", "doc")); err != nil {
			t.Fatal(err)
		}
		refs, err := st.ScanPathReferencesAmong(out, []string{out})
		if err != nil {
			t.Fatal(err)
		}
		if len(refs) != 1 || refs[0] != target {
			t.Fatalf("refs %v, want [%s]", refs, target)
		}
	})
}
