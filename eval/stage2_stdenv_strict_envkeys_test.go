package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestBootstrapStage2StdenvStrictEnvKeysMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	helper := filepath.Join(lay.Root, "eval", "testdata", "stage2_stdenv_drvattrs_for_strict.nix")
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	nixDrvPath := lay.EvalStrictDrvPath(t, string(raw))
	nixATerm := lay.RealiseDrvInStore(t, "", nixDrvPath)
	if len(nixATerm) == 0 {
		t.Fatalf("materialise nix drv %s", nixDrvPath)
	}
	nd, err := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}

	ev := NewEvaluator().WithEffects(IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(helper)
	expr, err := parser.ParseFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	attrsV, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	attrsV, err = ev.ForceValue(attrsV)
	if err != nil {
		t.Fatal(err)
	}
	drv, err := ev.AttrsToDerivation(attrsV)
	if err != nil {
		t.Fatal(err)
	}
	_, goHasDNBI := drv.Env["defaultNativeBuildInputs"]
	_, nixHasDNBI := nd.Env["defaultNativeBuildInputs"]
	_, goHasJSON := drv.Env["__json"]
	_, nixHasJSON := nd.Env["__json"]
	if goHasDNBI != nixHasDNBI {
		t.Fatalf("defaultNativeBuildInputs in env: go=%v nix=%v keys go=%d nix=%d", goHasDNBI, nixHasDNBI, len(drv.Env), len(nd.Env))
	}
	if goHasJSON != nixHasJSON {
		t.Fatalf("__json in env: go=%v nix=%v", goHasJSON, nixHasJSON)
	}
	if goHasDNBI && nixHasDNBI {
		if drv.Env["defaultNativeBuildInputs"] != nd.Env["defaultNativeBuildInputs"] {
			t.Fatalf("defaultNativeBuildInputs env differs go len=%d nix len=%d",
				len(drv.Env["defaultNativeBuildInputs"]), len(nd.Env["defaultNativeBuildInputs"]))
		}
	}
}
