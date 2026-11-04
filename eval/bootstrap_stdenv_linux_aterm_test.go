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

func TestBootstrapStage0StdenvLinuxATermMatchesNix(t *testing.T) {
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
	const suffix = "bootstrap-stage0-stdenv-linux.drv"
	var goKey string
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			goKey = k
			break
		}
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
		if strings.HasSuffix(k, suffix) {
			nixKey = k
			break
		}
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
			if len(nd.Env[k]) < 400 && len(gp.Env[k]) < 400 {
				t.Logf("  nix: %q", nd.Env[k])
				t.Logf("  go:  %q", gp.Env[k])
			}
		}
	}
	t.Fatalf("ATerm mismatch nix len=%d go len=%d", len(nixATerm), len(goATerm))
}
