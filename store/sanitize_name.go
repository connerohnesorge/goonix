// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"regexp"
	"strings"
	"unicode"
)

var storePathNameOK = regexp.MustCompile(`^[a-zA-Z0-9+._?=-]+$`)

// SanitizeStorePathName maps arbitrary text to a valid Nix store path name component
// (lib.strings.sanitizeDerivationName / Nix store-api).
func SanitizeStorePathName(name string) string {
	if name == "" {
		return "unknown"
	}
	if len(name) <= 207 && storePathNameOK.MatchString(name) {
		return name
	}
	s := strings.TrimLeft(name, ".")
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if isStorePathNameRune(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := b.String()
	if len(out) > 207 {
		out = out[len(out)-207:]
	}
	if out == "" {
		return "unknown"
	}
	return out
}

func isStorePathNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '+' || r == '.' || r == '_' || r == '?' || r == '=' || r == '-'
}
