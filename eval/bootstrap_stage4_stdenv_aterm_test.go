package eval_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func bootstrapStageStdenvATerm(t *testing.T, stage string) {
	t.Helper()
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	probe := filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix")
	goPath, ev := evalProbeDrvPath(t, probe)
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
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
	assertClosureKeySetsEqual(t, nixDoc, goDoc)
	suffix := "bootstrap-" + stage + "-stdenv-linux.drv"
	matched := drvshow.MatchKeysBySuffix(nixDoc, goDoc, suffix)
	sort.Strings(matched)
	if len(matched) == 0 {
		t.Fatalf("no exact key pairs for %s", suffix)
	}
	for _, key := range matched {
		bootstrapATermOracleKeys(t, key, key, ev)
	}
}

func TestBootstrapStage2StdenvLinuxATermMatchesNix(t *testing.T) {
	bootstrapStageStdenvATerm(t, "stage2")
}

func TestStdenvNoCCProbeClosureKeySetsMatchNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
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
	assertClosureKeySetsEqual(t, nixDoc, goDoc)
	if drvshow.DrvKey(nixPath) != drvshow.DrvKey(goPath) {
		t.Fatalf("root drv key nix=%q go=%q", drvshow.DrvKey(nixPath), drvshow.DrvKey(goPath))
	}
}
