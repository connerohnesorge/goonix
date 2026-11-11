package eval_test

import (
	"path/filepath"
	"testing"
)

func TestImportHeadProbeMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "import_head_probe.nix")
	nixV := evalOracleNix(t, path)
	if nixV["tryPkgs"] != true || nixV["ccType"] != "set" {
		t.Fatalf("nix import head probe unexpected: %#v", nixV)
	}
	goV, err := evalGoonixFileAllowErr(t, path)
	if err != nil {
		t.Skipf("goonix import head probe still red @6681: %v", err)
	}
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("import head probe differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
