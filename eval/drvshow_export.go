// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/store/drvshow"
)

// RegisteredDerivation returns a derivation registered by derivationStrict.
func (ev *Evaluator) RegisteredDerivation(drvPath string) (*gonixdrv.Derivation, bool) {
	if ev == nil || drvPath == "" {
		return nil, false
	}
	if ev.strictDrvs != nil {
		if d, ok := ev.strictDrvs[drvPath]; ok {
			return d, true
		}
	}
	if ev.moduloCache != nil {
		return ev.moduloCache.Lookup(drvPath)
	}
	return nil, false
}

// StrictDerivationCount is derivations recorded at the end of derivationStrict.
func (ev *Evaluator) StrictDerivationCount() int {
	if ev == nil || ev.strictDrvs == nil {
		return 0
	}
	return len(ev.strictDrvs)
}

// ModuloCacheCount returns derivations registered during evaluation (includes modulo-only entries).
func (ev *Evaluator) ModuloCacheCount() int {
	if ev == nil || ev.moduloCache == nil {
		return 0
	}
	return ev.moduloCache.Count()
}

// DerivationShowFromCache serialises every derivationStrict registration (debug aid).
func (ev *Evaluator) DerivationShowFromCache() drvshow.Document {
	doc := drvshow.Document{
		Derivations: make(map[string]drvshow.Entry),
		Version:     4,
	}
	if ev == nil || ev.strictDrvs == nil {
		return doc
	}
	for path, d := range ev.strictDrvs {
		doc.Derivations[drvshow.DrvKey(path)] = drvshow.FromDerivation(d)
	}
	return doc
}

// DerivationShowClosure walks inputDrvs from rootDrvPath using only derivationStrict registrations.
func (ev *Evaluator) DerivationShowClosure(rootDrvPath string) (drvshow.Document, error) {
	load := func(path string) (*gonixdrv.Derivation, error) {
		if d, ok := ev.RegisteredDerivation(path); ok {
			return d, nil
		}
		return nil, fmt.Errorf("derivation %q was not registered by derivationStrict (inputDrvs reference missing strict eval)", path)
	}
	return drvshow.BuildClosure(rootDrvPath, load)
}
