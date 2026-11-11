package parser

import (
	"strings"
	"testing"
)

func literalString(e Expr) string {
	switch x := e.(type) {
	case *String:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp != nil {
				b.WriteString("<interp>")
			} else {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	case *ConcatStrings:
		var b strings.Builder
		for _, p := range x.Parts {
			b.WriteString(literalString(p))
		}
		return b.String()
	default:
		return ""
	}
}

func TestStripIndentationS2Literal(t *testing.T) {
	src := "''  If the string starts with whitespace\n    followed by a newline, it's stripped, but\n    that's not the case here. Two spaces are\n    stripped because of the \"  \" at the start. \n  ''"
	e, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	got := literalString(e)
	line2 := strings.Split(got, "\n")[1]
	if !strings.HasPrefix(line2, "  followed") {
		t.Fatalf("line2 = %q", line2)
	}
}
