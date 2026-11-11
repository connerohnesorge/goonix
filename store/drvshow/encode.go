// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"sort"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// FromDerivation builds one nix-show entry from a parsed .drv.
func FromDerivation(d *gonixdrv.Derivation) Entry {
	name := d.Name()
	env := make(map[string]string, len(d.Env))
	for k, v := range d.Env {
		env[k] = NormalizePathValue(v)
	}
	outs := make(map[string]Output, len(d.Outputs))
	for outName, o := range d.Outputs {
		p := o.Path
		if p == "" {
			p = d.Env[outName]
		}
		outs[outName] = Output{
			Path:     StoreFingerprint(p),
			HashAlgo: o.HashAlgorithm,
			Hash:     o.Hash,
		}
	}
	drvs := make(map[string]InputDrv, len(d.InputDerivations))
	drvPaths := make([]string, 0, len(d.InputDerivations))
	for p := range d.InputDerivations {
		drvPaths = append(drvPaths, p)
	}
	sort.Strings(drvPaths)
	for _, p := range drvPaths {
		outNames := append([]string(nil), d.InputDerivations[p]...)
		sort.Strings(outNames)
		drvs[DrvKey(p)] = InputDrv{
			DynamicOutputs: map[string]any{},
			Outputs:        outNames,
		}
	}
	srcs := append([]string(nil), d.InputSources...)
	sort.Strings(srcs)
	for i, s := range srcs {
		srcs[i] = StoreFingerprint(s)
	}
	args := append([]string(nil), d.Arguments...)
	for i, a := range args {
		args[i] = NormalizePathValue(a)
	}
	return Entry{
		Args:    args,
		Builder: NormalizePathValue(d.Builder),
		Env:     env,
		Inputs: Inputs{
			Drvs: drvs,
			Srcs: srcs,
		},
		Name:    name,
		Outputs: outs,
		System:  d.Platform,
		Version: 4,
	}
}
