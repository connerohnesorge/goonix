// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package local

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/connerohnesorge/goonix/store"
)

// checkEntryName refuses a NAR entry name Windows cannot hold as written:
// reserved characters, device names, and names Windows would change by
// stripping a trailing dot or space. These are NovaNix's rules.
func checkEntryName(name string) error {
	if !utf8.ValidString(name) {
		return fmt.Errorf("NAR entry name '%s' is not valid UTF-8", name)
	}
	if strings.Contains(name, store.CaseHackSuffix) {
		return fmt.Errorf("NAR entry name contains the case-hack suffix: %s", name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 0x20 || strings.IndexByte(`<>:"\|?*`, c) >= 0 {
			return fmt.Errorf("NAR entry name '%s' contains a character Windows does not allow", name)
		}
	}
	if isReservedDeviceName(name) {
		return fmt.Errorf("NAR entry name '%s' is a Windows device name", name)
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("NAR entry name '%s' ends in a dot or a space, which Windows strips", name)
	}
	return nil
}

// isReservedDeviceName reports whether Windows treats name as a device: CON,
// PRN, AUX, NUL, COM0-9 or LPT0-9 (also with superscript digits), with any
// extension and with trailing spaces before it.
func isReservedDeviceName(name string) bool {
	stem := name
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	stem = strings.ToUpper(strings.TrimRight(stem, " "))
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(stem) > 3 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) {
		switch stem[3:] {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
			return true
		}
	}
	return false
}

func symlinkError(l pendingLink, err error) error {
	return fmt.Errorf("cannot create symlink %s -> %s: %v (on Windows this needs Developer Mode or elevation)", l.path, l.target, err)
}
