// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// ValidateDerivationName checks Nix store path name rules for derivation names.
func ValidateDerivationName(name string) error {
	if name == "." || name == ".." {
		return fmt.Errorf("invalid derivation name %q", name)
	}
	if strings.HasSuffix(name, ".drv") {
		return fmt.Errorf("invalid derivation name %q: must not end in .drv", name)
	}
	if len(name) > 211 {
		return fmt.Errorf("invalid derivation name %q: length %d exceeds 211", name, len(name))
	}
	for i, r := range name {
		if r == '"' || r == '\\' {
			return fmt.Errorf("invalid derivation name: name %q contains illegal character %q", name, string(r))
		}
		if !isStorePathNameRune(r) {
			return fmt.Errorf("invalid derivation name: name %q contains illegal character %q. Please pass a different 'name'.", name, string(name[i]))
		}
	}
	return nil
}

// ValidateOutputName checks output id strings (out, dev, etc.).
func ValidateOutputName(name string) error {
	return ValidateDerivationName(name)
}

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

// NameFromDerivationValue returns the logical derivation name for validation.
func NameFromDerivationValue(env map[string]string, structured bool, jsonEnv string) string {
	if structured {
		return structuredNameFromJSON(jsonEnv)
	}
	if env == nil {
		return ""
	}
	return env["name"]
}

func ensureStructuredName(d *gonixdrv.Derivation) {
	_ = derivationName(d)
}

func isStorePathNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '.' || r == '_' || r == '?' || r == '=' || r == '-'
}
