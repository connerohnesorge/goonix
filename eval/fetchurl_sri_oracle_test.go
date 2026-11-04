// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

func TestFetchurlSRIOutputHashAlgoEmptyMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	root := modRoot(t)
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	nix := filepath.Join(root, "testdata", "nix")
	src := `
let
  fetchurl = import <nix/fetchurl.nix>;
in (fetchurl {
  url = "https://ftpmirror.gnu.org/bison/bison-3.8.2.tar.gz";
  hash = "sha256-BsnhO99+sk1M62tZIFpPZ8LH5yExGWREMP6C+9FKCrs=";
}).drvPath
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + nix)
	ev.SetCurFile("fetchurl_sri_oracle.nix")
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	goPath := forceStringOrPath(t, ev, fv)
	gd, ok := ev.RegisteredDerivation(goPath)
	if !ok {
		t.Fatalf("go drv %q not in strict registry", goPath)
	}
	if gd.Env["outputHashAlgo"] != "" {
		t.Fatalf("outputHashAlgo=%q want empty for SRI", gd.Env["outputHashAlgo"])
	}
	if gd.Env["builder"] != "builtin:fetchurl" {
		t.Fatalf("env builder=%q", gd.Env["builder"])
	}
	if gd.Env["system"] != "builtin" {
		t.Fatalf("system=%q", gd.Env["system"])
	}

	nixCmd := exec.Command("nix-instantiate", "--eval", "--raw", "-")
	nixCmd.Stdin = strings.NewReader(src)
	nixCmd.Env = append(os.Environ(), "NIXPATH=nixpkgs="+nixpkgs+":nix="+nix)
	nixOut, err := nixCmd.Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	wantPath := strings.TrimSpace(string(nixOut))
	if goPath != wantPath {
		t.Logf("drvPath still differs (remaining fetchurl env): go=%q nix=%q", goPath, wantPath)
	}

	goDoc, err := ev.DerivationShowClosure(goPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := goDoc.Derivations[drvshow.DrvKey(goPath)]
	if entry.Env["outputHashAlgo"] != "" {
		t.Fatalf("show env outputHashAlgo=%q", entry.Env["outputHashAlgo"])
	}
}
