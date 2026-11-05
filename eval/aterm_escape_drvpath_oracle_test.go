package eval_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

type atermEscapeCase struct {
	name  string
	attrs string
}

func TestATermEscapeDrvPathMatchesNixDerivation(t *testing.T) {
	cases := []atermEscapeCase{
		{name: "arg_quote", attrs: `{ name = "esc-arg-quote"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "a\"b" ]; }`},
		{name: "arg_backslash", attrs: `{ name = "esc-arg-bs"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "back\\slash" ]; }`},
		{name: "arg_newline", attrs: "{ name = \"esc-arg-nl\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; args = [ \"line\\nbreak\" ]; }"},
		{name: "arg_tab", attrs: "{ name = \"esc-arg-tab\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; args = [ \"tab\\there\" ]; }"},
		{name: "arg_cr", attrs: "{ name = \"esc-arg-cr\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; args = [ \"cr\\rbreak\" ]; }"},
		{name: "arg_mixed", attrs: "{ name = \"esc-arg-mix\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; args = [ \"a\\\"b\\nc\\td\" \"\\\\\" ]; }"},
		{name: "env_value_quote", attrs: `{ name = "esc-env-q"; system = "x86_64-linux"; builder = "/bin/sh"; FOO = "a\"b"; }`},
		{name: "env_value_escapes", attrs: "{ name = \"esc-env-mix\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; FOO = \"x\\ny\\tz\"; }"},
	}
	lay := oracle.MustLayout(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nixPath := lay.EvalDerivationDrvPath(t, tc.attrs)
			goPath, _ := goDerivationStrictOracle(t, "", tc.attrs)
			if nixPath != goPath {
				t.Fatalf("drvPath\nnix=%q\ngo=%q", nixPath, goPath)
			}
		})
	}
}

func TestATermEscapeBytesMatchNixTempStore(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	attrs := "{ name = \"esc-bytes\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; args = [ \"a\\\"b\" \"\\n\\t\\r\\\\\" ]; FOO = \"v\\\"x\"; }"
	lay := oracle.MustLayout(t)
	nixPath := lay.EvalDerivationDrvPathInStore(t, storeRoot, attrs)
	lay.InstantiateStrictInStore(t, storeRoot, attrs)
	nixATerm := lay.ReadStoreDrvPath(t, storeRoot, nixPath)
	goPath, goATerm := goDerivationStrictOracle(t, storeRoot, attrs)
	if goPath != nixPath {
		t.Fatalf("drvPath go=%q nix=%q", goPath, nixPath)
	}
	if !bytes.Equal(nixATerm, goATerm) {
		t.Fatalf("ATerm bytes differ (%d vs %d)\nnix=%q\ngo=%q", len(nixATerm), len(goATerm), string(nixATerm), string(goATerm))
	}
}

func TestATermEscapeEnvKeyWhenNixAllows(t *testing.T) {
	// Nix rejects some env key shapes; probe once and compare when accepted.
	attrs := "{ name = \"esc-key\"; system = \"x86_64-linux\"; builder = \"/bin/sh\"; \"k\\t\" = \"v\"; }"
	lay := oracle.MustLayout(t)
	nixPath, err := lay.EvalDerivationDrvPathAllowError(attrs)
	if err != nil {
		t.Skipf("nix rejects env key escapes: %v", err)
	}
	goPath, _ := goDerivationStrictOracle(t, "", attrs)
	if nixPath != goPath {
		t.Fatalf("drvPath\nnix=%q\ngo=%q", nixPath, goPath)
	}
}
