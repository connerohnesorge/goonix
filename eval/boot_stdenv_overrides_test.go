package eval_test

import (
	"path/filepath"
	"testing"
)

func TestBootStdenvOverridesOracleMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_stdenv_overrides.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("boot stdenv overrides oracle:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
