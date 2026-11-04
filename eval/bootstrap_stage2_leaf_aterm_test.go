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

func storePathToDrvKey(p string) string {
	p = strings.TrimPrefix(p, "/nix/store/")
	return p + ".drv"
}

func bootstrapStage2GCCWrapperPair(t *testing.T) (goKey, nixKey string, ev *eval.Evaluator) {
	t.Helper()
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	const suffix = "bootstrap-stage2-gcc-wrapper-13.3.0.drv"
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			goKey = k
			break
		}
	}
	if goKey == "" {
		t.Fatal("go closure missing stage2 gcc-wrapper")
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
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			nixKey = k
			break
		}
	}
	if nixKey == "" {
		t.Fatal("nix closure missing stage2 gcc-wrapper")
	}
	return goKey, nixKey, ev
}

func bootstrapATermOracleKeys(t *testing.T, nixKey, goKey string, ev *eval.Evaluator) {
	t.Helper()
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatalf("go derivation not registered: %s", goKey)
	}
	goATerm, err := eval.SerializeGoNixDerivation(gd)
	if err != nil {
		t.Fatal(err)
	}
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(nixATerm, goATerm) {
		return
	}
	nd, err := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}
	gp := gd
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
			if len(nd.Env[k]) < 400 && len(gp.Env[k]) < 400 {
				t.Logf("  nix: %q", nd.Env[k])
				t.Logf("  go:  %q", gp.Env[k])
			}
		}
	}
	t.Fatalf("ATerm mismatch nix=%s go=%s", nixKey, goKey)
}

func bootstrapATermViaGCCWrapperEnv(t *testing.T, envName string) {
	t.Helper()
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	goGW, nixGW, ev := bootstrapStage2GCCWrapperPair(t)
	nixGWATerm, err := os.ReadFile("/nix/store/" + nixGW)
	if err != nil {
		t.Fatal(err)
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goGW)
	if !ok {
		t.Fatal("gcc-wrapper not registered")
	}
	goGWATerm, err := eval.SerializeGoNixDerivation(gd)
	if err != nil {
		t.Fatal(err)
	}
	nd, err := gonixdrv.ReadDerivation(bytes.NewReader(nixGWATerm))
	if err != nil {
		t.Fatal(err)
	}
	gp, err := gonixdrv.ReadDerivation(bytes.NewReader(goGWATerm))
	if err != nil {
		t.Fatal(err)
	}
	pickLeaf := func(d *gonixdrv.Derivation, needle string) string {
		for p := range d.InputDerivations {
			if strings.Contains(p, needle) {
				return drvshow.DrvKey(p)
			}
		}
		return ""
	}
	var nixLeafKey, goLeafKey string
	switch envName {
	case "cc":
		nixLeafKey = pickLeaf(nd, "xgcc")
		goLeafKey = pickLeaf(gp, "xgcc")
	case "bintools":
		nixLeafKey = pickLeaf(nd, "binutils-wrapper")
		goLeafKey = pickLeaf(gp, "binutils-wrapper")
	default:
		t.Fatalf("unknown env %q", envName)
	}
	bootstrapATermOracleKeys(t, nixLeafKey, goLeafKey, ev)
}

func TestBootstrapStage2XgccATermMatchesNix(t *testing.T) {
	bootstrapATermViaGCCWrapperEnv(t, "cc")
}

func TestBootstrapStage2BinutilsWrapperATermMatchesNix(t *testing.T) {
	bootstrapATermViaGCCWrapperEnv(t, "bintools")
}
