package ladder_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// buildGoonix compiles cmd/goonix once per test into dir.
func buildGoonix(t *testing.T, root, dir string) string {
	t.Helper()
	bin := filepath.Join(dir, "goonix")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/goonix")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

var pinnedArgs = []string{"--argstr", "system", "x86_64-linux", "--arg", "config", "{}", "--arg", "overlays", "[]"}

// TestInstantiateClosureMatchesGolden runs `goonix instantiate <nixpkgs> -A
// attr` for each issue #9 target and checks every .drv it writes against the
// committed sha256 manifest from nix-instantiate 2.24.9. It needs no Nix.
func TestInstantiateClosureMatchesGolden(t *testing.T) {
	root := ladderRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	bin := buildGoonix(t, root, t.TempDir())
	for _, attr := range []string{"hello", "stdenv", "bash", "coreutils"} {
		t.Run(attr, func(t *testing.T) {
			wantRaw, err := os.ReadFile(filepath.Join(root, "testdata", "golden", "closure", attr+".sha256"))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Split(strings.TrimSpace(string(wantRaw)), "\n")

			st := t.TempDir()
			args := append([]string{"instantiate", "--store", st, nixpkgs, "-A", attr}, pinnedArgs...)
			cmd := exec.Command(bin, args...)
			cmd.Env = append(os.Environ(), "NIX_PATH=", "HOME=/homeless-shelter")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("goonix instantiate: %v\n%s", err, out)
			}

			files, err := filepath.Glob(filepath.Join(st, "store", "*.drv"))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatal(err)
				}
				sum := sha256.Sum256(b)
				got = append(got, hex.EncodeToString(sum[:])+"  "+filepath.Base(f))
			}
			sort.Slice(got, func(i, j int) bool { return got[i][66:] < got[j][66:] })
			sort.Slice(want, func(i, j int) bool { return want[i][66:] < want[j][66:] })

			gotSet := map[string]bool{}
			for _, g := range got {
				gotSet[g] = true
			}
			bad := 0
			for _, w := range want {
				if !gotSet[w] {
					bad++
					if bad <= 10 {
						t.Errorf("missing or different: %s", w)
					}
				}
			}
			if len(got) != len(want) {
				t.Errorf("closure size: goonix %d, nix %d", len(got), len(want))
			}
			top := strings.TrimSpace(string(out))
			drvPath, _ := os.ReadFile(filepath.Join(root, "testdata", "golden", attr+".drvPath"))
			if attr == "hello" && top != strings.TrimSpace(string(drvPath)) {
				t.Errorf("printed %q, golden %q", top, strings.TrimSpace(string(drvPath)))
			}
		})
	}
}

// TestEvalLibVersionMatchesGolden is the issue #9 --eval check.
func TestEvalLibVersionMatchesGolden(t *testing.T) {
	root := ladderRoot(t)
	bin := buildGoonix(t, root, t.TempDir())
	want, err := os.ReadFile(filepath.Join(root, "testdata", "golden", "lib.version"))
	if err != nil {
		t.Fatal(err)
	}
	args := append([]string{"eval", filepath.Join(root, "testdata", "nixpkgs"), "-A", "lib.version"}, pinnedArgs...)
	out, err := exec.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("goonix eval: %v\n%s", err, out)
	}
	if got := strings.Trim(strings.TrimSpace(string(out)), `"`); got != strings.TrimSpace(string(want)) {
		t.Fatalf("lib.version = %q, want %q", got, want)
	}
}
