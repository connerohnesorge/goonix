package eval_test

import "testing"

func TestCallPackageWithXorgDefaultSelfCallPackage(t *testing.T) {
	t.Skip("generated self // { callPackage = 123; } aborts in Nix 2.24.9 before select; not a goonix-only regression")
}
