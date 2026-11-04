// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// NixClosure runs `nix derivation show` on drvPath and parses JSON.
func NixClosure(drvPath string) (Document, error) {
	if _, err := exec.LookPath("nix"); err != nil {
		return Document{}, fmt.Errorf("nix not on PATH")
	}
	out, err := exec.Command("nix", "derivation", "show", "--recursive", drvPath).Output()
	if err != nil {
		if len(out) > 0 {
			return Document{}, fmt.Errorf("nix derivation show: %w\n%s", err, out)
		}
		return Document{}, fmt.Errorf("nix derivation show: %w", err)
	}
	var doc Document
	if err := json.Unmarshal(out, &doc); err != nil {
		return Document{}, err
	}
	return doc, nil
}

// MaterializeNixDrv runs nix-instantiate on a .nix file that evaluates to a derivation.
func MaterializeNixDrv(nixFile string, env []string) (string, error) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		return "", fmt.Errorf("nix-instantiate not on PATH")
	}
	cmd := exec.Command("nix-instantiate", nixFile)
	cmd.Env = env
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(string(b)), `"`), nil
}
