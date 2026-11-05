package eval_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestDerivationStrictAtermEscapeOracle(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	attrs := `{
	  name = "aterm-escape";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "a\"b" "tab\there" "back\\slash" ];
	}`
	lay := oracle.MustLayout(t)
	nixPath := lay.EvalStrictDrvPathInStore(t, storeRoot, attrs)
	nixATerm := lay.RealiseDrvInStore(t, storeRoot, nixPath)
	if len(nixATerm) == 0 {
		t.Skip("could not materialise nix .drv in temp store (needs nix-store --realise)")
	}
	_, goATerm := goDerivationStrictOracle(t, storeRoot, attrs)
	if string(nixATerm) != string(goATerm) {
		t.Fatalf("ATerm mismatch vs nix\nnix=%q\ngo=%q", string(nixATerm), string(goATerm))
	}
}
