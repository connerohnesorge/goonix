package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestParseOkayIndStringLangCase(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "lang", "parse-okay-ind-string.nix")
	if _, err := parser.ParseFile(path); err != nil {
		t.Fatal(err)
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
