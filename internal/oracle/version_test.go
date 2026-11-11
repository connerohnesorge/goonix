// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"os/exec"
	"strings"
	"testing"
)

// DetectNixVersion returns the nix-instantiate version string (e.g. "2.24.9").
func DetectNixVersion() (string, error) {
	out, err := exec.Command("nix-instantiate", "--version").Output()
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(out))
	if i := strings.LastIndex(s, " "); i >= 0 {
		return strings.TrimSpace(s[i+1:]), nil
	}
	return s, nil
}

func TestPinnedNixVersionWhenPresent(t *testing.T) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
	v, err := DetectNixVersion()
	if err != nil {
		t.Fatal(err)
	}
	if v != NixVersion {
		RequireNix224(t)
	}
}
