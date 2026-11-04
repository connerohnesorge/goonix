// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"encoding/json"
	"fmt"
)

// ParseDerivationShowJSON accepts Nix 2.24.x output (top-level map of drvPath → entry)
// and 2.26+ / C++ API format ({ "derivations": {…}, "version": 4 }).
func ParseDerivationShowJSON(out []byte) (Document, error) {
	var probe struct {
		Derivations map[string]Entry `json:"derivations"`
		Version     int              `json:"version"`
	}
	if err := json.Unmarshal(out, &probe); err == nil && (probe.Derivations != nil || probe.Version != 0) {
		if probe.Derivations == nil {
			probe.Derivations = map[string]Entry{}
		}
		return Document{Derivations: probe.Derivations, Version: probe.Version}, nil
	}
	var legacy map[string]Entry
	if err := json.Unmarshal(out, &legacy); err != nil {
		return Document{}, fmt.Errorf("derivation show JSON: %w", err)
	}
	if len(legacy) == 0 {
		return Document{}, fmt.Errorf("derivation show JSON: empty document")
	}
	return Document{Derivations: legacy, Version: 0}, nil
}
