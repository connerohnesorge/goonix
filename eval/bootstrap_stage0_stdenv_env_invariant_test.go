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
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func bootstrapStage0StdenvGo(t *testing.T) (goKey string, gd *gonixdrv.Derivation, ev *eval.Evaluator) {
	t.Helper()
	root := modRoot(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	const suffix = "bootstrap-stage0-stdenv-linux.drv"
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			goKey = k
			break
		}
	}
	if goKey == "" {
		t.Fatal("go closure missing stage0 stdenv")
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatal("not registered")
	}
	return goKey, gd, ev
}

func bootstrapStage0StdenvNixATerm(t *testing.T) (nixKey string, nd *gonixdrv.Derivation) {
	t.Helper()
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
	const suffix = "bootstrap-stage0-stdenv-linux.drv"
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			nixKey = k
			break
		}
	}
	if nixKey == "" {
		t.Fatal("nix closure missing stage0 stdenv")
	}
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	nd, err = gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}
	return nixKey, nd
}

// TestBootstrapStage0StdenvEnvCoercionMatchesNixATerm checks each .drv env value byte-for-byte vs Nix.
func TestBootstrapStage0StdenvEnvCoercionMatchesNixATerm(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	_, gd, _ := bootstrapStage0StdenvGo(t)
	_, nd := bootstrapStage0StdenvNixATerm(t)
	keys := map[string]struct{}{}
	for k := range nd.Env {
		keys[k] = struct{}{}
	}
	for k := range gd.Env {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		gv, gok := gd.Env[k]
		nv, nok := nd.Env[k]
		if !gok || !nok {
			t.Fatalf("env[%q] presence go=%v nix=%v", k, gok, nok)
		}
		if gv != nv {
			t.Fatalf("env[%q] byte mismatch go len=%d nix len=%d", k, len(gv), len(nv))
		}
	}
}

func TestBootstrapStage0StdenvPreHookPreservesNewlines(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	_, gd, _ := bootstrapStage0StdenvGo(t)
	_, nd := bootstrapStage0StdenvNixATerm(t)
	gLines := strings.Count(gd.Env["preHook"], "\n")
	nLines := strings.Count(nd.Env["preHook"], "\n")
	if gLines != nLines {
		t.Fatalf("preHook newlines go=%d nix=%d", gLines, nLines)
	}
	if nLines == 0 {
		t.Fatal("expected newlines in nix preHook env")
	}
}

func TestBootstrapStage0StdenvPreHookAttrOracleMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "stage0_stdenv_prehook_oracle.nix")
	want := lay.EvalFileRaw(t, probe)
	got := evalGoonixFileRaw(t, probe)
	if got != want {
		t.Fatalf("stage0 preHook attr oracle\nnix=%s\ngo=%s", want, got)
	}
}

func evalGoonixFileRaw(t *testing.T, probe string) string {
	t.Helper()
	root := modRoot(t)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + filepath.Join(root, "testdata", "nixpkgs"))
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	ev.SetCurFile(probe)
	expr, err := parser.ParseFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	return forceStringOrPath(t, ev, val)
}

func TestBootstrapStage1StdenvEnvCoercionMatchesNixATerm(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	goKey, gd := bootstrapStageStdenvGoShared(t, "stage1")
	nixKey, nd := bootstrapStageStdenvNixShared(t, "stage1")
	if goKey != nixKey {
		t.Fatalf("stage1 stdenv drv key mismatch go=%s nix=%s", goKey, nixKey)
	}
	compareStdenvEnvMaps(t, "stage1", gd.Env, nd.Env)
}

func bootstrapStageStdenvNixKey(t *testing.T, stage string) string {
	t.Helper()
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
	rootKey := drvshow.DrvKey(nixPath)
	return pickBootstrapStdenvKey(t, nixDoc, rootKey, stage)
}

func pickBootstrapStdenvKey(t *testing.T, doc drvshow.Document, rootKey, stage string) string {
	t.Helper()
	suffix := "bootstrap-" + stage + "-stdenv-linux.drv"
	k := drvshow.PickKeyByBFS(doc, rootKey, func(k string, e drvshow.Entry) bool {
		return strings.HasSuffix(k, suffix)
	})
	if k == "" {
		t.Fatalf("closure missing %s", stage)
	}
	return k
}

func bootstrapStageStdenvGoShared(t *testing.T, stage string) (goKey string, gd *gonixdrv.Derivation) {
	t.Helper()
	nixKey := bootstrapStageStdenvNixKey(t, stage)
	root := modRoot(t)
	_, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	gd, ok := ev.RegisteredDerivation("/nix/store/" + nixKey)
	if !ok {
		t.Fatalf("go did not register nix-pinned stdenv %s (stage %s)", nixKey, stage)
	}
	// Detect duplicate logical stdenv registrations with wrong hashes.
	goDoc, err := ev.DerivationShowClosure("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	suffix := "bootstrap-" + stage + "-stdenv-linux.drv"
	var dupes []string
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) && k != nixKey {
			dupes = append(dupes, k)
		}
	}
	if len(dupes) > 0 {
		t.Fatalf("go closure has extra %q nodes besides nix %s: %v", suffix, nixKey, dupes)
	}
	return nixKey, gd
}

func bootstrapStageStdenvNixShared(t *testing.T, stage string) (nixKey string, nd *gonixdrv.Derivation) {
	t.Helper()
	nixKey = bootstrapStageStdenvNixKey(t, stage)
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	nd, err = gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}
	return nixKey, nd
}

func compareStdenvEnvMaps(t *testing.T, stage string, goEnv, nixEnv map[string]string) {
	t.Helper()
	keys := map[string]struct{}{}
	for k := range nixEnv {
		keys[k] = struct{}{}
	}
	for k := range goEnv {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	for _, k := range sorted {
		gv, gok := goEnv[k]
		nv, nok := nixEnv[k]
		if !gok || !nok {
			t.Fatalf("stage %s env[%q] presence go=%v nix=%v", stage, k, gok, nok)
		}
		if gv != nv {
			if k == "defaultNativeBuildInputs" && len(gv) < 200 && len(nv) < 200 {
				t.Fatalf("stage %s env[%q]\n  go:  %q\n  nix: %q", stage, k, gv, nv)
			}
			t.Fatalf("stage %s env[%q] byte mismatch go len=%d nix len=%d", stage, k, len(gv), len(nv))
		}
	}
}
