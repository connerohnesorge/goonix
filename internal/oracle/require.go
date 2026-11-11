// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// RequireNix224 skips the test, with a reason, when nix-instantiate is not
// Nix 2.24.9. The ladder job sets GOONIX_REQUIRE_NIX_224=1 so that skip
// becomes a failure instead of a silent pass.
func RequireNix224(t *testing.T) {
	t.Helper()
	out, err := exec.Command("nix-instantiate", "--version").CombinedOutput()
	ver := strings.TrimSpace(string(out))
	if err != nil || !strings.Contains(ver, NixVersion) {
		reason := fmt.Sprintf("oracle requires Nix %s, found %q", NixVersion, ver)
		if os.Getenv("GOONIX_REQUIRE_NIX_224") == "1" {
			t.Fatal(reason)
		}
		t.Skip(reason)
	}
}
