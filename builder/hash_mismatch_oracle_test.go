// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestBuildHashMismatchMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	const attrs = `{
	  name = "h";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "printf hi > \"$out\"" ];
	  outputHashMode = "flat";
	  outputHashAlgo = "sha256";
	  outputHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=";
	}`
	err = realiseAttrs(t, st, attrs, true)
	if err == nil {
		t.Fatal("fixed-output build accepted the wrong hash")
	}
	msg := err.Error()
	if !strings.Contains(msg, "hash mismatch in fixed-output derivation '") || !strings.Contains(msg, "\n  specified: sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\n     got:    sha256-") {
		t.Fatalf("go mismatch text:\n%s", msg)
	}
	if _, lookErr := exec.LookPath("nix-build"); lookErr != nil {
		oracle.RequireNix224(t)
		t.Fatal("nix-build is not on PATH")
	}
	oracle.RequireNix224(t)
	cmd := exec.Command("nix-build",
		"--option", "substituters", "",
		"--option", "sandbox", "false",
		"-E", "derivation "+attrs)
	out, _ := cmd.CombinedOutput()
	nixText := string(out)
	if !strings.Contains(nixText, "hash mismatch in fixed-output derivation '") {
		t.Fatalf("nix-build:\n%s", nixText)
	}
	for _, line := range strings.Split(msg, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.Contains(trim, "specified:") && !strings.Contains(trim, "got:") {
			continue
		}
		if !strings.Contains(nixText, trim) {
			t.Fatalf("nix-build is missing %q\nnix:\n%s\ngo:\n%s", trim, nixText, msg)
		}
	}
}
