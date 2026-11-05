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

func TestBootstrapXgccStrictInputDrvsMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	helper := filepath.Join(lay.Root, "eval", "testdata", "xgcc_drvattrs_for_strict.nix")
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
	if !reflect.DeepEqual(drv.InputDerivations, nd.InputDerivations) {
		for p, goOuts := range drv.InputDerivations {
			nixOuts := nd.InputDerivations[p]
			if !reflect.DeepEqual(goOuts, nixOuts) {
				t.Errorf("inputDrvs[%q] go=%v nix=%v", p, goOuts, nixOuts)
			}
		}
		for p, nixOuts := range nd.InputDerivations {
			if _, ok := drv.InputDerivations[p]; !ok {
				t.Errorf("inputDrvs[%q] only in nix=%v", p, nixOuts)
			}
		}
		t.Fatal("inputDrvs differ")
	}
}

func scanValueStringContext(ev *Evaluator, v *Value, col *derivationInputCollector) error {
	if v == nil {
		return nil
	}
	fv, err := ev.forceValue(v)
	if err != nil {
		return err
	}
	switch fv.Kind {
	case VKString:
		col.mergeValueContext(fv)
	case VKList:
		for _, el := range fv.List {
			if err := scanValueStringContext(ev, el, col); err != nil {
				return err
			}
		}
	case VKAttrs:
		for sym := range fv.Attrs.Binds {
			b := fv.Attrs.Binds[sym]
			el, err := ev.attrBindingValue(b)
			if err != nil {
				return err
			}
			if err := scanValueStringContext(ev, el, col); err != nil {
				return err
			}
		}
	}
	return nil
}
