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
		if strings.Contains(string(raw), "is not implemented") {
			t.Fatalf("%s still contains a builtin stub (\"is not implemented\")", ent.Name())
		}
	}
}

