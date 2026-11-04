package eval

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestUnescapeIndentedStringDollarBrace(t *testing.T) {
	got := unescapeIndentedString(`''${foo}`)
	if got != "${foo}" {
		t.Fatalf("got %q", got)
	}
}

func TestIndentedStringStripAndUnescapeMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `let s = ''
    ''${x}-ok
  ''; in s`
	want := lay.EvalStdinRaw(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ev.forceString(fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
