package eval_test

import (
	"testing"
)

func TestOrFallbackNullUsesRight(t *testing.T) {
	// Nix strict: ({ a = null; }.a or 456) → null; missing-attr or is SelectOr (see TestOrFallbackMissingAttrMatchesNix).
	t.Skip("cursed `null or 7` is not Nix strict semantics")
}
