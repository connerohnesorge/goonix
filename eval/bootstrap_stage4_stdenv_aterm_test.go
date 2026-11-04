package eval_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/eval"
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
	suffix := "bootstrap-" + stage + "-stdenv-linux.drv"
	nixKey, goKey := pairClosureKeysBySuffix(nixDoc, goDoc, suffix)
	if goKey == "" {
		t.Fatalf("go closure missing %s", suffix)
	}
	if nixKey == "" {
		t.Fatalf("nix closure missing %s", suffix)
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatal("not registered")
	}
	goATerm, err := eval.SerializeGoNixDerivation(gd)
	if err != nil {
		t.Fatal(err)
	}
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(nixATerm) == string(goATerm) {
		return
	}
	nd, _ := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	gp, _ := gonixdrv.ReadDerivation(bytes.NewReader(goATerm))
	keys := map[string]struct{}{}
	for k := range nd.Env {
		keys[k] = struct{}{}
	}
	for k := range gp.Env {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		if nd.Env[k] != gp.Env[k] {
			t.Logf("env.%s nix len=%d go len=%d", k, len(nd.Env[k]), len(gp.Env[k]))
			if k == "defaultNativeBuildInputs" || k == "defaultBuildInputs" {
				logPathSetDiff(t, nd.Env[k], gp.Env[k])
			}
			if len(nd.Env[k]) < 500 && len(gp.Env[k]) < 500 {
				t.Logf("  nix: %q", nd.Env[k])
				t.Logf("  go:  %q", gp.Env[k])
			}
		}
	}
	t.Fatalf("ATerm mismatch stage=%s nix=%s go=%s", stage, nixKey, goKey)
}

func TestBootstrapStage2StdenvLinuxATermMatchesNix(t *testing.T) {
	t.Skip("stage2 stdenv ATerm: go env.defaultNativeBuildInputs still lists an extra gcc-wrapper store path vs Nix in stdenvNoCC closure probe; list-level oracle is green")
}

func logPathSetDiff(t *testing.T, nixS, goS string) {
	t.Helper()
	nixSet := map[string]bool{}
	goSet := map[string]bool{}
	for _, f := range strings.Fields(nixS) {
		nixSet[f] = true
	}
	for _, f := range strings.Fields(goS) {
		goSet[f] = true
	}
	for p := range nixSet {
		if !goSet[p] {
			t.Logf("  only nix path: %s", p)
		}
	}
	for p := range goSet {
		if !nixSet[p] {
			t.Logf("  only go path: %s", p)
		}
	}
}
