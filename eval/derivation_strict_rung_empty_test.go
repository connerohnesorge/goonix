package eval_test

import (
	"testing"
)

func TestDerivationStrictRung05IgnoreNullsEmptyString(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung05-empty"; system = "x86_64-linux"; builder = "/bin/sh"; __ignoreNulls = true; skip = null; empty = ""; keep = "yes"; }`)
}
