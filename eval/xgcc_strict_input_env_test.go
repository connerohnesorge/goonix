package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestBootstrapXgccStrictInputEnvMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	helper := filepath.Join(lay.Root, "eval", "testdata", "xgcc_drvattrs_for_strict.nix")
	raw, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	nixDrvPath := lay.EvalStrictDrvPath(t, string(raw))
	nixATerm, err := os.ReadFile(nixDrvPath)
	if err != nil {
		t.Fatal(err)
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
	if !reflect.DeepEqual(drv.Env, nd.Env) {
		for k := range nd.Env {
			if drv.Env[k] != nd.Env[k] {
				t.Fatalf("full env[%q] differs go len=%d nix len=%d", k, len(drv.Env[k]), len(nd.Env[k]))
			}
		}
		for k := range drv.Env {
			if _, ok := nd.Env[k]; !ok {
				t.Fatalf("env[%q] only in go", k)
			}
		}
	}
}
