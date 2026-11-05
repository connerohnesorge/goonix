// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

func structuredNameFromJSON(raw string) string {
	if raw == "" {
		return ""
	}
	var meta struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(raw), &meta); err != nil {
		return ""
	}
	return meta.Name
}

func derivationName(d *gonixdrv.Derivation) string {
	if d == nil {
		return ""
	}
	if n := d.Name(); n != "" {
		return n
	}
	return structuredNameFromJSON(d.Env["__json"])
}

func ensureStructuredName(d *gonixdrv.Derivation) {
	_ = derivationName(d)
}
