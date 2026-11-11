package eval_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestXgccEnvBytesVsNixATerm(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	nixKey, goKey, ev, _ := bootstrapStage2XgccPair(t)
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatal("go xgcc not registered")
	}
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
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
	var first string
	for _, k := range sorted {
		gv, gok := gd.Env[k]
		nv, nok := nd.Env[k]
		if !gok || !nok {
			if gok {
				t.Logf("env[%q] only in go: %q", k, gv)
			}
			if nok {
				t.Logf("env[%q] only in nix: %q", k, nv)
			}
			if first == "" {
				first = k
			}
			continue
		}
		if gv != nv {
			if first == "" {
				first = k
			}
			t.Logf("env[%q] go len=%d nix len=%d", k, len(gv), len(nv))
			if len(gv) < 200 && len(nv) < 200 {
				t.Logf("  go:  %q", gv)
				t.Logf("  nix: %q", nv)
			}
		}
	}
	if first != "" {
		t.Fatalf("first env mismatch: %s", first)
	}
}

func TestTexinfoEnvBytesVsNixATermPaired(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	nixXgcc, goXgcc, ev, _ := bootstrapStage2XgccPair(t)
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
	_ = nixXgcc
	goDoc, err := ev.DerivationShowClosure("/nix/store/" + goXgcc)
	if err != nil {
		t.Fatal(err)
	}
	const suffix = "texinfo-7.1.1.drv"
	var goKey, nixKey string
	for k, e := range goDoc.Derivations {
		if strings.HasSuffix(k, suffix) && strings.Contains(e.Env["stdenv"], "bootstrap-stage1-stdenv-linux") {
			goKey = k
			break
		}
	}
	if goKey == "" {
		t.Fatal("missing go texinfo stage1")
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goKey)
	if !ok {
		t.Fatal("not registered")
	}
	for k, e := range nixDoc.Derivations {
		if strings.HasSuffix(k, suffix) && strings.Contains(e.Env["stdenv"], "bootstrap-stage1-stdenv-linux") {
			nixKey = k
			break
		}
	}
	if nixKey == "" {
		t.Fatal("missing nix texinfo stage1")
	}
	nixATerm, err := os.ReadFile("/nix/store/" + nixKey)
	if err != nil {
		t.Fatal(err)
	}
	nd, err := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
	if err != nil {
		t.Fatal(err)
	}
	if gd.Env["buildInputs"] != nd.Env["buildInputs"] {
		t.Fatalf("buildInputs\ngo:  %q\nnix: %q", gd.Env["buildInputs"], nd.Env["buildInputs"])
	}
}
