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
