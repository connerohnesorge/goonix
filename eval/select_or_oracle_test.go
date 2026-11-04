package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestSelectOrMissingAttrMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	for _, src := range []string{
		`toString ({}.missing or 123)`,
		`toString ({ a = null; }.a or 456)`,
		`toString (let f = k: { "1" = 10; }.${k} or "99"; in f "2")`,
	} {
		want := lay.EvalRaw(t, src)
		expr, err := parser.ParseString(src)
		if err != nil {
			t.Fatal(err)
		}
		ev := eval.NewEvaluator()
		val, err := ev.EvalWithBuiltins(expr)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		got := forceStringOrPath(t, ev, val)
		if got != want {
			t.Fatalf("%q got %q want %q", src, got, want)
		}
	}
}
