package eval_test

import (
	"path/filepath"
	"testing"
)

func TestBootOnlyStdenvProbeMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_only_stdenv_probe.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("boot-only stdenv probe differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
	if nixV["ccType"] != "set" {
		t.Fatalf("nix ccType want set got %v", nixV["ccType"])
	}
}
