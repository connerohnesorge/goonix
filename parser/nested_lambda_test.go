package parser

import (
	"fmt"
	"testing"
)

func TestLexNestedLambda(t *testing.T) {
	lr, err := lex([]byte(`x: def: let a = 1; in a`), "(string)")
	if err != nil {
		t.Fatal(err)
	}
	for i, tok := range lr.tokens {
		if tok.Type == tokenEOF {
			break
		}
		fmt.Printf("%d: %v %q\n", i, tok.Type, lr.slice(tok.Start, tok.End))
	}
}

func TestParseNestedLambdaLet(t *testing.T) {
	expr, err := ParseString(`x: def: let a = 1; in a`)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(Print(expr))
}
