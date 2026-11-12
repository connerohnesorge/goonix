// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"strings"
)

// CaseHackSuffix marks a store entry whose name was changed so it could sit
// next to a sibling that differs from it only in case. On a case-insensitive
// store (Windows) the NAR is written with the suffix removed, and a NAR whose
// names collide case-insensitively is restored with the suffix added, as
// Nix's use-case-hack and NovaNix do.
const CaseHackSuffix = "~nix~case~hack~"

// stripCaseHack removes a trailing CaseHackSuffix and its number from name.
func stripCaseHack(name string) string {
	i := strings.LastIndex(name, CaseHackSuffix)
	if i < 0 {
		return name
	}
	digits := name[i+len(CaseHackSuffix):]
	if digits == "" {
		return name
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return name
		}
	}
	return name[:i]
}
