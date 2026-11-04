package eval_test

import (
	"path/filepath"
	"testing"
)

func TestBootOnlyTryEvalMatchesNix(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "boot_only_tryeval.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("boot tryEval nix=%v go=%v", nixV, goV)
	}
}
