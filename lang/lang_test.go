package lang_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestParseOkayLangCases(t *testing.T) {
	root := modRoot(t)
	dir := filepath.Join(root, "testdata", "lang")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("no testdata/lang")
	}
	var n int
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "parse-okay-") || !strings.HasSuffix(e.Name(), ".nix") {
			continue
		}
		n++
		path := filepath.Join(dir, e.Name())
		_, err := parser.ParseFile(path)
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
	if n == 0 {
		t.Skip("no parse-okay cases")
	}
}

func modRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		p := filepath.Dir(wd)
		if p == wd {
			t.Fatal("no go.mod")
		}
		wd = p
	}
}
