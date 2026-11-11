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

func TestGlibcBootstrapFilesATermMatchesNix(t *testing.T) {
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
	const suffix = "bootstrap-stage0-glibc-bootstrapFiles.drv"
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
	if string(nixATerm) != string(goATerm) {
		i := 0
		for i < len(nixATerm) && i < len(goATerm) && nixATerm[i] == goATerm[i] {
			i++
		}
		endN, endG := i+120, i+120
		if endN > len(nixATerm) {
			endN = len(nixATerm)
		}
		if endG > len(goATerm) {
			endG = len(goATerm)
		}
		start := i - 30
		if start < 0 {
			start = 0
		}
		nd, _ := gonixdrv.ReadDerivation(bytes.NewReader(nixATerm))
		gdParsed, _ := gonixdrv.ReadDerivation(bytes.NewReader(goATerm))
		for _, k := range sortedEnvKeys(nd.Env, gdParsed.Env) {
			if nd.Env[k] != gdParsed.Env[k] {
				t.Logf("env.%s differs nix len=%d go len=%d", k, len(nd.Env[k]), len(gdParsed.Env[k]))
				if k == "buildCommand" || len(nd.Env[k]) < 300 {
					t.Logf("  nix: %q", nd.Env[k])
					t.Logf("  go:  %q", gdParsed.Env[k])
				}
			}
		}
		t.Fatalf("ATerm mismatch at %d (nix len=%d go len=%d)\nnix: %q\ngo:  %q",
			i, len(nixATerm), len(goATerm),
			string(nixATerm[start:endN]),
			string(goATerm[start:endG]))
	}
}

func sortedEnvKeys(a, b map[string]string) []string {
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
