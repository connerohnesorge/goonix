// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func findGoDrvPathByLogical(t *testing.T, ev *eval.Evaluator, rootDrvPath, logical string) string {
	t.Helper()
	doc, err := ev.DerivationShowClosure(rootDrvPath)
	if err != nil {
		t.Fatal(err)
	}
	for k := range doc.Derivations {
		if strings.HasSuffix(k, "-"+logical+".drv") {
			return "/nix/store/" + k
		}
	}
	return ""
}

func TestPkgConfigFetchurlGoATermDebug(t *testing.T) {
	if os.Getenv("GOONIX_DEBUG_PKG") == "" {
		t.Skip("set GOONIX_DEBUG_PKG=1")
	}
	root := modRoot(t)
	goRoot, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	goPath := findGoDrvPathByLogical(t, ev, goRoot, "pkg-config-0.29.2.tar.gz")
	if goPath == "" {
		t.Fatal("not found")
	}
	d, ok := ev.RegisteredDerivation(goPath)
	if !ok {
		t.Fatal("not registered")
	}
	aterm, err := eval.SerializeGoNixDerivation(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("goPath=%q", goPath)
	for _, k := range []string{"outputHash", "outputHashAlgo", "outputHashMode", "out", "urls", "url", "preferLocalBuild", "impureEnvVars"} {
		t.Logf("env[%s]=%q", k, d.Env[k])
	}
	for out, o := range d.Outputs {
		t.Logf("output %s hashAlgo=%q hash=%q path=%q", out, o.HashAlgorithm, o.Hash, o.Path)
	}
	t.Logf("ATerm=%s", string(aterm))
	_ = drvshow.FromDerivation(d)
}

func TestPkgConfigFetchurlNixATermDebug(t *testing.T) {
	if os.Getenv("GOONIX_DEBUG_PKG") == "" {
		t.Skip("set GOONIX_DEBUG_PKG=1")
	}
	lay := oracle.MustLayout(t)
	root := modRoot(t)
	nixPath, err := drvshow.MaterializeNixDrv(lay, filepath.Join(root, "eval", "testdata", "stdenv_nocc_inst.nix"))
	if err != nil {
		t.Fatal(err)
	}
	key := "6b29gjz7rj4mw0ch0vy2m6qrqipz2bbb-pkg-config-0.29.2.tar.gz.drv"
	t.Logf("nixPath=%s key=%s", nixPath, key)
	if b, err := os.ReadFile("/nix/store/" + key); err == nil {
		t.Logf("ATerm=%s", string(b))
	}
}
