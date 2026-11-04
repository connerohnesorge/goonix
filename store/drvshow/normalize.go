// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"path"
	"strings"
)

const storePrefix = "/nix/store/"

// Basename returns the store path base (hash-name) without directory.
func Basename(storePath string) string {
	if storePath == "" {
		return ""
	}
	storePath = strings.TrimSuffix(storePath, ".drv")
	return path.Base(storePath)
}

// StoreFingerprint strips /nix/store/ leaving hash-name (no .drv).
func StoreFingerprint(storePath string) string {
	s := strings.TrimPrefix(storePath, storePrefix)
	s = strings.TrimSuffix(s, ".drv")
	return s
}

// DrvKey is the map key used in nix derivation show (basename of .drv path).
func DrvKey(drvPath string) string {
	if drvPath == "" {
		return ""
	}
	if !strings.HasSuffix(drvPath, ".drv") {
		drvPath += ".drv"
	}
	return path.Base(drvPath)
}

// NameStdenvPairID pairs derivations by pkg name and the logical stdenv drv (env.stdenv).
func NameStdenvPairID(e Entry) string {
	stage := ""
	if v := e.Env["stdenv"]; v != "" {
		stage = LogicalName(v)
	}
	return e.Name + "\x1e" + stage
}

// LogicalName is the human-readable suffix after the store hash (e.g. bash-5.3p20).
func LogicalName(storePathOrDrv string) string {
	fp := StoreFingerprint(drvPathKey(storePathOrDrv))
	if i := strings.Index(fp, "-"); i >= 0 && i+1 < len(fp) {
		return fp[i+1:]
	}
	return fp
}

func drvPathKey(p string) string {
	if strings.HasSuffix(p, ".drv") {
		return p
	}
	if strings.Contains(path.Base(p), ".drv") {
		return p
	}
	return p + ".drv"
}

// NormalizePathValue converts a store path to show fingerprint form.
func NormalizePathValue(v string) string {
	if strings.HasPrefix(v, storePrefix) {
		return StoreFingerprint(v)
	}
	return v
}
