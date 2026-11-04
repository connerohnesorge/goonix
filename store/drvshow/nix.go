// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/connerohnesorge/goonix/internal/oracle"
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

// MaterializeNixDrv runs hermetic nix-instantiate on a .nix file (see internal/oracle).
func MaterializeNixDrv(l oracle.Layout, nixFile string) (string, error) {
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		return "", fmt.Errorf("nix-instantiate not on PATH")
	}
	args := append(append([]string{}, l.InstantiateArgs()...), nixFile)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	b, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(string(b)), `"`), nil
}
