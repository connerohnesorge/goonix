// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestWrapperMissingInputsDebug(t *testing.T) {
	if os.Getenv("GOONIX_DEBUG_WRAPPERS") == "" {
		t.Skip("set GOONIX_DEBUG_WRAPPERS=1")
	}
	root := modRoot(t)
	lay := oracle.MustLayout(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	nixPath, err := drvshow.MaterializeNixDrv(lay, filepath.Join(root, "eval", "testdata", "stdenv_nocc_inst.nix"))
	if err != nil {
		t.Fatal(err)
	}
	nixDoc, err := drvshow.NixClosure(nixPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gcc-wrapper-13.3.0", "binutils-wrapper-2.43.1", "binutils-patchelfed-ld-wrapper-2.43.1"} {
		var nixKey, goKey string
		for k := range nixDoc.Derivations {
			if strings.HasSuffix(k, "-"+want+".drv") {
				nixKey = k
				break
			}
		}
		for k := range goDoc.Derivations {
			if strings.HasSuffix(k, "-"+want+".drv") {
				goKey = k
				break
			}
		}
		t.Logf("want=%q nixKey=%q goKey=%q registered=%v", want, nixKey, goKey, goKey != "")
	}
	// stdenv-linux parent
	const logical = "stdenv-linux"
	var nixStd, goStd string
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, "-"+logical+".drv") && !strings.Contains(k, "bootstrap") {
			nixStd = k
		}
	}
	for k := range goDoc.Derivations {
		if strings.HasSuffix(k, "-"+logical+".drv") && !strings.Contains(k, "bootstrap") {
			goStd = k
		}
	}
	t.Logf("stdenv nix=%q go=%q", nixStd, goStd)
	if nixStd != "" {
		ne := nixDoc.Derivations[nixStd]
		for dp := range ne.Inputs.Drvs {
			if strings.Contains(dp, "gcc-wrapper-13.3.0") || strings.Contains(dp, "binutils-wrapper") {
				t.Logf("nix stdenv inputDrvs %q -> %v", dp, ne.Inputs.Drvs[dp])
			}
		}
		for k, v := range ne.Env {
			if strings.Contains(v, "gcc-wrapper-13.3.0") || strings.Contains(v, "binutils-wrapper-2.43") {
				t.Logf("nix stdenv env[%s]=%q", k, v[:min(160, len(v))])
			}
		}
	}
	if goStd != "" {
		ge := goDoc.Derivations[goStd]
		for dp := range ge.Inputs.Drvs {
			if strings.Contains(dp, "wrapper") {
				t.Logf("go stdenv inputDrvs %q -> %v", dp, ge.Inputs.Drvs[dp])
			}
		}
		for k, v := range ge.Env {
			if strings.Contains(v, "wrapper") {
				t.Logf("go stdenv env[%s]=%q", k, v[:min(160, len(v))])
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
