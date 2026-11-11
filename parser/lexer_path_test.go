package parser_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
)

func TestParseRejectBareSlash(t *testing.T) {
	for _, src := range []string{`1 == /`, `[ / ]`, `/`} {
		_, err := parser.ParseString(src)
		if err == nil {
			t.Fatalf("expected parse error for %q", src)
		}
	}
}

func TestParseOkaySlashDot(t *testing.T) {
	_, err := parser.ParseString(`/.`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = parser.ParseString(`let x = /. + "/foo"; in x`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(`/.`, " ") {
		t.Fatal("sanity")
	}
}
