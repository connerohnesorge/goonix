package eval_test

import (
	"testing"
)

func TestDerivationStrictRung09DepOnly(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }`)
}
