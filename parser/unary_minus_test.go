package parser

import "testing"

func TestParseUnaryMinusInParens(t *testing.T) {
	expr, err := ParseString(`builtins.sub (-1)`)
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if got == "" {
		t.Fatal("empty print")
	}
}
