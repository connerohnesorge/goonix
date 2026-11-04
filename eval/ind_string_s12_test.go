package eval_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestEvalOkayIndStringS12MatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	raw, err := os.ReadFile(filepath.Join(modRoot(t), "testdata/lang/eval-okay-ind-string.nix"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)(let.*?s12 = ''.*?'';)`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("no let through s12")
	}
	src := m[1] + " in s12"
	want := lay.EvalStdinRawBytes(t, src)
	expr, err := parser.ParseString(src)
	if err != nil {
	 t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
