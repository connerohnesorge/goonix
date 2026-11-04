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

func assertClosureKeySetsEqual(t *testing.T, nixDoc, goDoc drvshow.Document) {
	t.Helper()
	nixKeys := drvshow.ClosureKeySet(nixDoc)
	goKeys := drvshow.ClosureKeySet(goDoc)
	if len(nixKeys) != len(goKeys) {
		t.Fatalf("closure size nix=%d go=%d", len(nixKeys), len(goKeys))
	}
	for i := range nixKeys {
		if nixKeys[i] != goKeys[i] {
			t.Fatalf("closure key mismatch at %d: nix=%q go=%q", i, nixKeys[i], goKeys[i])
		}
	}
}

func bootstrapATermOracleLogical(t *testing.T, logicalSuffix string) {
	t.Helper()
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
	var goKey, nixKey, logical string
	var nixCandidates []string
	for k := range nixDoc.Derivations {
		ln := drvshow.LogicalName(k)
		if ln == logicalSuffix || strings.HasSuffix(ln, logicalSuffix) {
			nixCandidates = append(nixCandidates, k)
		}
	}
	sort.Strings(nixCandidates)
	for _, nk := range nixCandidates {
		ne := nixDoc.Derivations[nk]
		pid := drvshow.NameStdenvPairID(ne)
		for gk, ge := range goDoc.Derivations {
			if drvshow.NameStdenvPairID(ge) == pid {
				nixKey = nk
				goKey = gk
				logical = drvshow.LogicalName(nk)
				break
			}
		}
		if goKey != "" {
			break
		}
	}
	if goKey == "" {
		t.Fatalf("go closure missing logical suffix %s (pair with nix)", logicalSuffix)
	}
	if nixKey == "" {
		t.Fatalf("nix closure missing logical %q", logical)
	}
	bootstrapATermOracleKeys(t, nixKey, goKey, ev)
}

func pairClosureKeysBySuffix(nixDoc, goDoc drvshow.Document, suffix string) (nixKey, goKey string) {
	matched := drvshow.MatchKeysBySuffix(nixDoc, goDoc, suffix)
	sort.Strings(matched)
	if len(matched) > 0 {
		return matched[0], matched[0]
	}
	// Fallback when store hashes differ but logical+stdenv pairing is needed (rare).
	var nixCandidates []string
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			nixCandidates = append(nixCandidates, k)
		}
	}
	sort.Strings(nixCandidates)
	for _, nk := range nixCandidates {
		pid := drvshow.NameStdenvPairID(nixDoc.Derivations[nk])
		for gk, ge := range goDoc.Derivations {
			if !strings.HasSuffix(gk, suffix) {
				continue
			}
			if drvshow.NameStdenvPairID(ge) == pid {
				return nk, gk
			}
		}
	}
	if len(nixCandidates) > 0 {
		nixKey = nixCandidates[0]
	}
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			goKey = k
			break
		}
	}
	return nixKey, goKey
}

func bootstrapATermOracle(t *testing.T, drvSuffix string) {
	t.Helper()
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
	var goKey string
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, drvSuffix) {
			goKey = k
			break
		}
	}
	if goKey == "" {
		t.Fatalf("go closure missing %s", drvSuffix)
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatal("not registered")
	}
	goATerm, err := eval.SerializeGoNixDerivation(gd)
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
	var nixKey string
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, drvSuffix) {
			nixKey = k
			break
		}
	}
	if nixKey == "" {
		t.Fatalf("nix closure missing %s", drvSuffix)
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
			t.Logf("env.%q nix len=%d go len=%d", k, len(nd.Env[k]), len(gp.Env[k]))
			if len(nd.Env[k]) < 400 && len(gp.Env[k]) < 400 {
				t.Logf("  nix: %q", nd.Env[k])
				t.Logf("  go:  %q", gp.Env[k])
			}
		}
	}
	t.Fatalf("ATerm mismatch suffix=%s nix=%s go=%s", drvSuffix, nixKey, goKey)
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
			t.Logf("env.%q nix len=%d go len=%d", k, len(nd.Env[k]), len(gp.Env[k]))
			if len(nd.Env[k]) < 400 && len(gp.Env[k]) < 400 {
				t.Logf("  nix: %q", nd.Env[k])
				t.Logf("  go:  %q", gp.Env[k])
			}
		}
	}
	t.Fatalf("ATerm mismatch nix=%s go=%s", nixKey, goKey)
}
