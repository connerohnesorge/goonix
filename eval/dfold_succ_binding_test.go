package eval_test

import (
	"path/filepath"
	"testing"
)

func TestDfoldSuccUsesCurBinding(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "dfold_succ_binding.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("dfold succ binding differs:\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
