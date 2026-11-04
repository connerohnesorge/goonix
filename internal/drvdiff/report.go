// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvdiff

import (
	"fmt"
	"sort"

	"github.com/connerohnesorge/goonix/store/drvshow"
)

// Report summarises a closure comparison.
type Report struct {
	Deepest     *Result
	ByteFirst   *ByteCulprit
	MissingInGo []string // logical drv names present in nix closure but not in go doc
}

// Analyse compares nix and go closures and returns deepest field mismatch plus missing go nodes.
func Analyse(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string) (Report, error) {
	return AnalyseWithATerm(nixDoc, goDoc, nixRootKey, goRootKey, nil)
}

// AnalyseWithATerm also runs leaf-first ATerm byte comparison when src is non-nil.
func AnalyseWithATerm(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string, src ATermBytes) (Report, error) {
	deepest, err := Compare(nixDoc, goDoc, nixRootKey, goRootKey)
	if err != nil {
		return Report{}, err
	}
	var byteFirst *ByteCulprit
	if src != nil {
		byteFirst, err = CompareBytesBottomUp(nixDoc, goDoc, nixRootKey, goRootKey, src)
		if err != nil {
			return Report{}, err
		}
	}
	nixNames := make(map[string]struct{})
	for k := range nixDoc.Derivations {
		nixNames[drvshow.LogicalName(k)] = struct{}{}
	}
	goNames := make(map[string]struct{})
	for k := range goDoc.Derivations {
		goNames[drvshow.LogicalName(k)] = struct{}{}
	}
	var missing []string
	for name := range nixNames {
		if _, ok := goNames[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return Report{Deepest: deepest, ByteFirst: byteFirst, MissingInGo: missing}, nil
}

// FormatReport prints human-readable analysis.
func FormatReport(r Report) string {
	var s string
	if len(r.MissingInGo) > 0 {
		s += fmt.Sprintf("go closure missing %d logical derivation(s) present in nix:\n", len(r.MissingInGo))
		for _, m := range r.MissingInGo {
			s += "  - " + m + "\n"
		}
	}
	s += FormatResult(r.Deepest)
	if r.ByteFirst != nil {
		s += FormatByteCulprit(r.ByteFirst)
	}
	return s
}
