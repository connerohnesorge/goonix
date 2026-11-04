// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

// RegisteredDerivation returns a derivation previously registered in the modulo cache.
func (ev *Evaluator) RegisteredDerivation(drvPath string) (*gonixdrv.Derivation, bool) {
	if ev == nil || ev.moduloCache == nil {
		return nil, false
	}
	return ev.moduloCache.Lookup(drvPath)
}

// ModuloCacheCount returns derivations registered during evaluation.
func (ev *Evaluator) ModuloCacheCount() int {
	if ev == nil || ev.moduloCache == nil {
		return 0
	}
	return ev.moduloCache.Count()
}

// DerivationShowFromCache serialises every registered derivation (not only root-reachable).
func (ev *Evaluator) DerivationShowFromCache() drvshow.Document {
	doc := drvshow.Document{
		Derivations: make(map[string]drvshow.Entry),
		Version:     4,
	}
	if ev == nil || ev.moduloCache == nil {
		return doc
	}
	ev.moduloCache.RangeRegistered(func(path string, d *gonixdrv.Derivation) bool {
		doc.Derivations[drvshow.DrvKey(path)] = drvshow.FromDerivation(d)
		return true
	})
	return doc
}

// DerivationShowClosure builds nix-show JSON for the closure reachable from rootDrvPath.
func (ev *Evaluator) DerivationShowClosure(rootDrvPath string) (drvshow.Document, error) {
	load := func(path string) (*gonixdrv.Derivation, error) {
		if d, ok := ev.RegisteredDerivation(path); ok {
			return d, nil
		}
		return drvshow.LoadFromFile(path)
	}
	return drvshow.BuildClosure(rootDrvPath, load)
}
