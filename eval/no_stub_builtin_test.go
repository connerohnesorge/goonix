package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoBuiltinStubBodies(t *testing.T) {
	dir := filepath.Join(modRoot(t), "eval")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") || strings.HasSuffix(ent.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			// Nix's own fetchTree error for an unknown input type.
			if strings.Contains(line, "cannot show unsupported input") {
				continue
			}
			low := strings.ToLower(line)
			if strings.Contains(low, "not implemented") || strings.Contains(low, "unsupported") {
				t.Fatalf("%s still contains a builtin stub: %s", ent.Name(), strings.TrimSpace(line))
			}
		}
	}
}

