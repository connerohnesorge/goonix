package eval_test

import (
	"testing"
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
}
