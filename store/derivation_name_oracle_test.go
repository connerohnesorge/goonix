package store_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store"
)

func TestValidateDerivationNameRejectsLikeNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	cases := []string{".", "..", "foo.drv", strings.Repeat("a", 212)}
	for _, name := range cases {
		attrs := `{ name = ` + quote(name) + `; builder = "x"; system = "x86_64-linux"; }`
		if _, err := lay.EvalDerivationDrvPathAllowError(attrs); err == nil {
			t.Fatalf("nix accepted name %q", name)
		}
		if err := store.ValidateDerivationName(name); err == nil {
			t.Fatalf("go accepted name %q", name)
		}
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}
