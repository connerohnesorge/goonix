package ladder_test

import (
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

// TestParsePrintMatchesNix compares goonix's parse printer with
// nix-instantiate --parse on every 25th .nix file of the pinned nixpkgs
// (about 1,500 files). All 36,860 matched when this test was written.
func TestParsePrintMatchesNix(t *testing.T) {
	root := ladderRoot(t)
	oracle.RequireNix224(t)
	if testing.Short() {
		t.Skip("runs nix-instantiate once per sampled file")
	}
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	var files []string
	n := 0
	err := filepath.WalkDir(nixpkgs, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".nix") {
			return err
		}
		if n%25 == 0 {
			files = append(files, p)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for _, f := range files {
		want, err := exec.Command("nix-instantiate", "--parse", f).Output()
		if err != nil {
			t.Fatalf("nix-instantiate --parse %s: %v", f, err)
		}
		expr, err := parser.ParseFile(f)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			bad++
			continue
		}
		if got := parser.Print(expr); got != strings.TrimSpace(string(want)) {
			bad++
			if bad <= 5 {
				t.Errorf("%s differs from nix-instantiate --parse", strings.TrimPrefix(f, nixpkgs+"/"))
			}
		}
	}
	if bad > 0 {
		t.Fatalf("%d of %d sampled files differ", bad, len(files))
	}
}
