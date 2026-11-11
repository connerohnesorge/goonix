package ladder_test

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestParityCasesMatchGolden runs NovaNix's drvPath case list
// (testdata/parity/cases.txt) through goonix instantiate and compares the
// printed path with nix-instantiate 2.24.9's (testdata/golden/parity.txt).
// It needs no Nix. The NixOS cases take several GB and skip under -short.
func TestParityCasesMatchGolden(t *testing.T) {
	root := ladderRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	want := readPairs(t, filepath.Join(root, "testdata", "golden", "parity.txt"))
	cases := readPairs(t, filepath.Join(root, "testdata", "parity", "cases.txt"))
	bin := buildGoonix(t, root, t.TempDir())
	for _, c := range cases {
		label, expr := c[0], c[1]
		t.Run(label, func(t *testing.T) {
			if testing.Short() && strings.HasPrefix(label, "nixos-") {
				t.Skip("NixOS evaluation needs several GB; runs without -short")
			}
			golden, ok := lookup(want, label)
			if !ok {
				t.Fatalf("no golden for %s", label)
			}
			dir := t.TempDir()
			file := filepath.Join(dir, "case.nix")
			src := `let pkgs = import <nixpkgs> { system = "x86_64-linux"; config = { }; overlays = [ ]; }; in ` + expr + "\n"
			if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bin, "instantiate", "--store", filepath.Join(dir, "store-root"), "-I", "nixpkgs="+nixpkgs, file)
			cmd.Env = append(os.Environ(), "NIX_PATH=", "HOME=/homeless-shelter")
			out, err := cmd.Output()
			if err != nil {
				msg := ""
				if ee, ok := err.(*exec.ExitError); ok {
					msg = string(ee.Stderr)
				}
				t.Fatalf("goonix instantiate: %v\n%s", err, msg)
			}
			if got := strings.TrimSpace(string(out)); got != golden {
				t.Fatalf("drvPath\n got %s\nwant %s", got, golden)
			}
		})
	}
}

// readPairs reads "label rest-of-line" lines, skipping blanks and comments.
func readPairs(t *testing.T, path string) [][2]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][2]string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		label, rest, _ := strings.Cut(line, " ")
		out = append(out, [2]string{label, strings.TrimSpace(rest)})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func lookup(pairs [][2]string, label string) (string, bool) {
	for _, p := range pairs {
		if p[0] == label {
			return p[1], true
		}
	}
	return "", false
}
