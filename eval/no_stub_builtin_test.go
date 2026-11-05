package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoBuiltinStubBodies(t *testing.T) {
	root := modRoot(t)
	for _, dir := range []string{"eval", "builder", "store"} {
		scanStubSources(t, filepath.Join(root, dir))
	}
}

func scanStubSources(t *testing.T, dir string) {
	t.Helper()
	err := filepath.WalkDir(dir, func(path string, ent os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ent.IsDir() {
			return nil
		}
		if !strings.HasSuffix(ent.Name(), ".go") || strings.HasSuffix(ent.Name(), "_test.go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(modRoot(t), path)
		for _, line := range strings.Split(string(raw), "\n") {
			// Nix's own fetchTree error for an unknown input type.
			if strings.Contains(line, "cannot show unsupported input") {
				continue
			}
			// The parser fallthrough keeps its honest message. The comment is the exemption.
			if strings.Contains(line, "stub-scan: exempt") {
				continue
			}
			low := strings.ToLower(line)
			if strings.Contains(low, "not implemented") || strings.Contains(low, "unsupported") {
				t.Fatalf("%s still contains a stub: %s", rel, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
