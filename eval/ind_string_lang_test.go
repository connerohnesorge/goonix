package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestEvalOkayIndStringMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	nixFile := filepath.Join(root, "testdata", "lang", "eval-okay-ind-string.nix")
	wantOracle := lay.EvalFileRawBytes(t, nixFile)

	expr, err := parser.ParseFile(nixFile)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != wantOracle {
		i := firstDiff(got, wantOracle)
		t.Fatalf("length got=%d want=%d first diff at %d\nnix: %q\ngo:  %q", len(got), len(wantOracle), i, snippet(wantOracle, i), snippet(got, i))
	}
}

func snippet(s string, at int) string {
	start := at - 20
	if start < 0 {
		start = 0
	}
	end := at + 40
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}

func firstDiff(a, b string) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return len(a)
		}
		return len(b)
	}
	return -1
}
