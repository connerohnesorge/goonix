// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"sort"
	"strings"
)

// NormalizeDocumentKeys rewrites map keys to basename .drv keys and normalizes input drv edges.
func NormalizeDocument(doc Document) Document {
	out := Document{
		Version:     doc.Version,
		Derivations: make(map[string]Entry, len(doc.Derivations)),
	}
	for k, e := range doc.Derivations {
		out.Derivations[DrvKey(k)] = normalizeEntryInputs(e)
	}
	return out
}

func normalizeEntryInputs(e Entry) Entry {
	if len(e.Inputs.Drvs) == 0 {
		return e
	}
	drvs := make(map[string]InputDrv, len(e.Inputs.Drvs))
	for k, v := range e.Inputs.Drvs {
		drvs[DrvKey(k)] = v
	}
	e.Inputs.Drvs = drvs
	return e
}

// MatchKeysBySuffix returns paired keys that exist in both documents with identical map keys.
func MatchKeysBySuffix(nixDoc, goDoc Document, suffix string) []string {
	var keys []string
	for k := range nixDoc.Derivations {
		if strings.HasSuffix(k, suffix) {
			if _, ok := goDoc.Derivations[k]; ok {
				keys = append(keys, k)
			}
		}
	}
	return keys
}

// ClosureKeySet returns sorted derivation show keys in doc.
func ClosureKeySet(doc Document) []string {
	keys := make([]string, 0, len(doc.Derivations))
	for k := range doc.Derivations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
