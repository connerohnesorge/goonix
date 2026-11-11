package eval_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestExpandResponseParamsATermMatchesNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	const suffix = "expand-response-params.drv"
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
	nixKey, goKey := pairClosureKeysBySuffix(nixDoc, goDoc, suffix)
	if goKey == "" {
		t.Fatal("missing go expand-response-params")
	}
	if nixKey == "" {
		t.Fatal("missing nix expand-response-params")
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
	if bytes.Equal(nixATerm, goATerm) {
		return
	}
	nd, err := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}
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
		if nd.Env[k] != gd.Env[k] {
			t.Logf("env.%q nix=%q go=%q", k, trunc(nd.Env[k], 120), trunc(gd.Env[k], 120))
		}
	}
	t.Logf("inputDrvs nix=%d go=%d", len(nd.InputDerivations), len(gd.InputDerivations))
	for p := range nd.InputDerivations {
		t.Logf("  nix input %s", drvshow.DrvKey(p))
	}
	for p := range gd.InputDerivations {
		t.Logf("  go input %s", drvshow.DrvKey(p))
	}
	t.Fatalf("ATerm mismatch nix=%s go=%s", nixKey, goKey)
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
