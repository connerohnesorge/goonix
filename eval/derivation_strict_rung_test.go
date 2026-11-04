package eval_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
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
	t.Skip("goonix multi-output drvPath vs Nix 2.24.9: env outputs ordering under investigation")
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
	t.Skip("builtins.toFile store path hash must match Nix for inputSrcs fingerprint")
	assertDerivationStrictRung(t, `{ name = "rung08-tofile"; system = "x86_64-linux"; builder = "/bin/sh"; doc = builtins.toFile "rung08-doc" "hello store"; }`)
}

func TestDerivationStrictRung09InputDrv(t *testing.T) {
	t.Skip("inputDrvs fingerprint: dep.out string context vs Nix (after rung09-dep path match)")
	assertDerivationStrictRung(t, `let dep = builtins.derivationStrict { name = "rung09-dep"; system = "x86_64-linux"; builder = "/bin/sh"; }; in {
	  name = "rung09-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  depOut = dep.out;
	}`)
}

func TestDerivationStrictRung10FixedOutput(t *testing.T) {
	t.Skip("fixed-output: SRI outputHash → go-nix Output hash fields")
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
	t.Helper()
	nixPath, _ := nixDerivationStrictOracle(t, attrs)
	goPath, goATerm := goDerivationStrictOracle(t, attrs)
	if nixPath != goPath {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", nixPath, goPath)
	}
	if err := assertATermMatchesDrvPath(t, goPath, goATerm); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(nixPath); err == nil {
		if !bytes.Equal(raw, goATerm) {
			t.Fatalf("ATerm mismatch vs nix store file\nnix=%q\ngo=%q", string(raw), string(goATerm))
		}
	}
}

func assertATermMatchesDrvPath(t *testing.T, wantPath string, aterm []byte) error {
	t.Helper()
	d, err := store.ReadDerivation(bytes.NewReader(aterm))
	if err != nil {
		return fmt.Errorf("parse go ATerm: %w", err)
	}
	got, err := store.DrvPath(d)
	if err != nil {
		return fmt.Errorf("DrvPath from ATerm: %w", err)
	}
	if got != wantPath {
		return fmt.Errorf("ATerm hashes to %q, want %q", got, wantPath)
	}
	return nil
}

func nixDerivationStrictOracle(t *testing.T, attrs string) (drvPath string, aterm []byte) {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(builtins.derivationStrict %s).drvPath`, strictArg)
	out, err := exec.Command("nix-instantiate", "--eval", "-E", pathExpr).Output()
	if err != nil {
		t.Fatalf("nix drvPath: %v", err)
	}
	drvPath = strings.Trim(strings.TrimSpace(string(out)), `"`)
	if raw, err := os.ReadFile(drvPath); err == nil {
		return drvPath, raw
	}
	return drvPath, nil
}

func goDerivationStrictOracle(t *testing.T, attrs string) (drvPath string, aterm []byte) {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	src := fmt.Sprintf(`let a = %s; in builtins.derivationStrict a`, strictArg)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
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
	drvPath, err = eval.ForceDerivationAttrs(ev, fv)
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
	drv.DrvPath = drvPath
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	aterm, err = eval.SerializeDerivation(cfg, drv)
	if err != nil {
		t.Fatal(err)
	}
	return drvPath, aterm
}
