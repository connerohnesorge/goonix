// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrokenWrapperNameInspect(t *testing.T) {
	if os.Getenv("GOONIX_DEBUG_WRAPPERS") == "" {
		t.Skip("set GOONIX_DEBUG_WRAPPERS=1")
	}
	root := modRoot(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	for path, entry := range goDoc.Derivations {
		if strings.Contains(path, "gcc-13.3.0.drv") && !strings.Contains(path, "wrapper") {
			t.Logf("gcc drv=%q env.name=%q env.pname=%q", path, entry.Env["name"], entry.Env["pname"])
		}
		if strings.Contains(path, "wrapper-13.3.0") || strings.Contains(path, "wrapper-2.43.1") {
			t.Logf("drv=%q env.name=%q", path, entry.Env["name"])
		}
	}
}
