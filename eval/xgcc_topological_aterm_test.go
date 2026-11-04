package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/drvdiff"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func bootstrapStage2XgccPair(t *testing.T) (nixKey, goKey string, ev *eval.Evaluator, goRootPath string) {
	t.Helper()
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	goRootPath, ev = evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goRootPath)
	if err != nil {
		t.Fatal(err)
	}
	const gwSuffix = "bootstrap-stage2-gcc-wrapper-13.3.0.drv"
	var goGW string
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, gwSuffix) {
			goGW = k
			break
		}
	}
	if goGW == "" {
		t.Fatal("go closure missing stage2 gcc-wrapper")
	}
	for dk := range goDoc.Derivations[goGW].Inputs.Drvs {
		if strings.Contains(drvshow.LogicalName(dk), "xgcc-13.3.0") {
			goKey = dk
			break
		}
	}
	if goKey == "" {
		t.Fatal("go gcc-wrapper missing xgcc input")
	}
	inst := filepath.Join(root, "eval", "testdata", "stdenv_nocc_inst.nix")
	nixPath, err := drvshow.MaterializeNixDrv(lay, inst)
	if err != nil {
		t.Fatal(err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		t.Fatal(err)
	}
	var nixGW string
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, gwSuffix) {
			nixGW = k
			break
		}
	}
	if nixGW == "" {
		t.Fatal("nix closure missing stage2 gcc-wrapper")
	}
	for dk := range nixDoc.Derivations[nixGW].Inputs.Drvs {
		if strings.Contains(drvshow.LogicalName(dk), "xgcc-13.3.0") {
			nixKey = dk
			break
		}
	}
	if nixKey == "" {
		t.Fatal("nix gcc-wrapper missing xgcc input")
	}
	return nixKey, goKey, ev, goRootPath
}

func TestBootstrapStage2XgccTopologicalByteFirstNameStdenv(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	nixXgcc, goXgcc, ev, goPath := bootstrapStage2XgccPair(t)
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	inst := filepath.Join(root, "eval", "testdata", "stdenv_nocc_inst.nix")
	nixPath, err := drvshow.MaterializeNixDrv(lay, inst)
	if err != nil {
		t.Fatal(err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		t.Fatal(err)
	}
	goFull, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	src := drvdiff.StoreATermSource{
		Go: func(goDrvKey string) []byte {
			gd, ok := ev.RegisteredDerivation("/nix/store/" + goDrvKey)
			if !ok {
				return nil
			}
			b, err := eval.SerializeGoNixDerivation(gd)
			if err != nil {
				return nil
			}
			return b
		},
	}
	culprit, err := drvdiff.CompareBytesBottomUpNameStdenv(nixDoc, goFull, nixXgcc, goXgcc, src)
	if err != nil {
		t.Fatal(err)
	}
	if culprit == nil {
		return
	}
	t.Log(drvdiff.FormatByteCulprit(culprit))
	t.Fatalf("xgcc closure byte-first culprit pair=%q", culprit.PairID)
}
