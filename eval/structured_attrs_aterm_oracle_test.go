package eval_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestStructuredAttrsStrictATermMatchesNix(t *testing.T) {
	attrs := `{
	  __structuredAttrs = true;
	  name = "structured-attrs-oracle";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  nested = { foo = 1; bar = [ "a" null ]; inner = { x = true; }; };
	}`
	assertDerivationStrictRung(t, attrs)
	checks := `{
	  __structuredAttrs = true;
	  name = "structured-attrs-checks";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  outputs = [ "out" "dev" ];
	  outputChecks = { out = { maxSize = 10; allowedReferences = [ ]; }; };
	  unsafeDiscardReferences = { out = false; dev = true; };
	  greeting = "hi";
	}`
	assertDerivationStrictRung(t, checks)
	lay := oracle.MustLayout(t)
	for _, attrs := range []string{attrs, checks} {
		nixOut := lay.EvalStrictOutPath(t, attrs)
		goOut := outPathOf(t, eval.NewEvaluator(), attrs)
		if goOut != nixOut {
			t.Fatalf("out path\ngo=%s\nnix=%s", goOut, nixOut)
		}
	}
}

func TestStructuredAttrsTopLevelWarningPrintsOnce(t *testing.T) {
	expr := `(derivation { name = "sa"; system = "x86_64-linux"; builder = "/bin/sh"; __structuredAttrs = true; allowedReferences = [ ]; }).drvPath`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	got := evalPkgsExprString(t, expr)
	w.Close()
	os.Stderr = old
	body, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	want := "warning: In a derivation named 'sa', 'structuredAttrs' disables the effect of the derivation attribute 'allowedReferences'; use 'outputChecks.<output>.allowedReferences' instead\n"
	if string(body) != want {
		t.Fatalf("stderr=%q want=%q drv=%s", body, want, got)
	}
	if strings.Count(string(body), "structuredAttrs") != 1 {
		t.Fatalf("warning printed %d times: %q", strings.Count(string(body), "structuredAttrs"), body)
	}
}
