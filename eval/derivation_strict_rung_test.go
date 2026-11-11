package eval_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

// derivationStrict rungs: drvPath and .drv ATerm bytes vs Nix 2.24.9.
func TestDerivationStrictRung01Minimal(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung01-minimal"; system = "x86_64-linux"; builder = "/bin/sh"; }`)
}

func TestDerivationStrictRung02Args(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung02-args"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "-c" "echo ok" ]; }`)
}

func TestDerivationStrictRung03ExtraEnv(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung03-env"; system = "x86_64-linux"; builder = "/bin/sh"; FOO = "bar"; count = 42; }`)
}

func TestDerivationStrictRung04MultiOutput(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung04-multi"; system = "x86_64-linux"; builder = "/bin/sh"; outputs = [ "out" "dev" ]; }`)
}

func TestDerivationStrictRung05IgnoreNulls(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung05-null"; system = "x86_64-linux"; builder = "/bin/sh"; __ignoreNulls = true; skip = null; keep = "yes"; }`)
}

func TestDerivationStrictRung06Coercions(t *testing.T) {
	assertDerivationStrictRung(t, `{ name = "rung06-coerce"; system = "x86_64-linux"; builder = "/bin/sh"; b = true; i = 7; lst = [ "a" "b" ]; }`)
}

func TestDerivationStrictRung07PathInput(t *testing.T) {
	root := modRoot(t)
	p := filepath.Join(root, "eval", "testdata", "derivation_strict", "seed.txt")
	assertDerivationStrictRung(t, fmt.Sprintf(`{
	  name = "rung07-path";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  src = %q;
	}`, p))
}

func TestDerivationStrictRung08ToFile(t *testing.T) {
	storeRoot := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	attrs := `{ name = "rung08-tofile"; system = "x86_64-linux"; builder = "/bin/sh"; doc = builtins.toFile "rung08-doc" "hello store"; }`
	assertDerivationStrictRungInStore(t, storeRoot, attrs)
}

func assertDerivationStrictDrvPathInStore(t *testing.T, storeRoot, attrs string) {
	t.Helper()
	nixPath := oracle.MustLayout(t).EvalStrictDrvPathInStore(t, storeRoot, attrs)
	goPath, _ := goDerivationStrictOracle(t, storeRoot, attrs)
	if nixPath != goPath {
		t.Fatalf("drvPath mismatch in store %q\nnix=%q\ngo=%q", storeRoot, nixPath, goPath)
	}
}

func TestDerivationStrictRung09InputDrv(t *testing.T) {
	assertDerivationStrictRung(t, `let dep = builtins.derivationStrict { name = "rung09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "rung09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`)
}

func TestDerivationStrictRung10FixedOutput(t *testing.T) {
	assertDerivationStrictRung(t, `{
	  name = "rung10-fixed";
	  system = "x86_64-linux";
	  builder = "builtin:fetchurl";
	  outputHashAlgo = "sha256";
	  outputHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
	  outputHashMode = "flat";
	}`)
}

func assertDerivationStrictRung(t *testing.T, attrs string) {
	assertDerivationStrictRungInStore(t, "", attrs)
}

func assertDerivationStrictRungInStore(t *testing.T, storeRoot, attrs string) {
	t.Helper()
	nixPath, nixATerm := nixDerivationStrictOracle(t, storeRoot, attrs)
	goPath, goATerm := goDerivationStrictOracle(t, storeRoot, attrs)
	if nixPath != goPath {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", nixPath, goPath)
	}
	if err := assertATermMatchesDrvPath(t, goPath, goATerm); err != nil {
		t.Fatal(err)
	}
	if len(nixATerm) > 0 && !bytes.Equal(nixATerm, goATerm) {
		t.Fatalf("ATerm mismatch vs nix store file\nnix=%q\ngo=%q", string(nixATerm), string(goATerm))
	}
}

func assertATermMatchesDrvPath(t *testing.T, wantPath string, aterm []byte) error {
	t.Helper()
	got, err := store.DrvPathFromATermBytes(store.Config{}, aterm)
	if err != nil {
		return fmt.Errorf("DrvPath from ATerm bytes: %w", err)
	}
	if got != wantPath {
		return fmt.Errorf("ATerm hashes to %q, want %q", got, wantPath)
	}
	return nil
}

func nixDerivationStrictOracle(t *testing.T, storeRoot, attrs string) (drvPath string, aterm []byte) {
	t.Helper()
	lay := oracle.MustLayout(t)
	drvPath = lay.EvalStrictDrvPathInStore(t, storeRoot, attrs)
	if storeRoot != "" {
		lay.InstantiateStrictInStore(t, storeRoot, attrs)
		return drvPath, lay.ReadStoreDrvPath(t, storeRoot, drvPath)
	}
	if st, err := os.Stat(drvPath); err == nil && !st.IsDir() {
		if raw, err := os.ReadFile(drvPath); err == nil {
			return drvPath, raw
		}
	}
	if raw := lay.RealiseDrvInStore(t, "", drvPath); len(raw) > 0 {
		return drvPath, raw
	}
	return drvPath, nil
}

func effectsForOracleStore(storeRoot string) eval.Effects {
	if storeRoot == "" {
		return eval.IOEffects{}
	}
	return eval.ChrootIOEffects{PhysRoot: filepath.Dir(storeRoot)}
}

func goDerivationStrictOracle(t *testing.T, storeRoot, attrs string) (drvPath string, aterm []byte) {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	src := fmt.Sprintf(`let a = %s; in builtins.derivationStrict a`, strictArg)
	ev := eval.NewEvaluator().WithEffects(effectsForOracleStore(storeRoot))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	drvPath, err = eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	attrsExpr, err := parser.ParseString(strictArg)
	if err != nil {
		t.Fatal(err)
	}
	attrsV, err := ev.EvalWithBuiltins(attrsExpr)
	if err != nil {
		t.Fatal(err)
	}
	av, err := ev.ForceValue(attrsV)
	if err != nil {
		t.Fatal(err)
	}
	drv, err := ev.AttrsToDerivation(av)
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	if err := eval.PrepareDerivationPaths(ev, cfg, drv); err != nil {
		t.Fatal(err)
	}
	if drvPath != drv.DrvPath {
		t.Fatalf("drvPath strict=%q computed=%q", drvPath, drv.DrvPath)
	}
	aterm, err = eval.SerializeDerivation(cfg, drv)
	if err != nil {
		t.Fatal(err)
	}
	return drvPath, aterm
}
