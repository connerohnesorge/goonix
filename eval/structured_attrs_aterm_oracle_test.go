package eval_test

import (
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
