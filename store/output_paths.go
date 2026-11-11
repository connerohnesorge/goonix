// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"sort"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// LogicalOutputPaths returns output store paths recorded on a derivation after
// evaluation (derivationStrict / .drv env). It does not recompute paths from
// the derivation ATerm (go-nix CalculateOutputPaths omits input-drv hashes and
// mishandles quoting in args).
func LogicalOutputPaths(d *gonixdrv.Derivation) (map[string]string, error) {
	if d == nil {
		return nil, fmt.Errorf("nil derivation")
	}
	names := make([]string, 0, len(d.Outputs))
	for name := range d.Outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make(map[string]string, len(names))
	for _, name := range names {
		o := d.Outputs[name]
		p := ""
		if o != nil {
			p = o.Path
		}
		if p == "" && d.Env != nil {
			p = d.Env[name]
		}
		if p == "" {
			return nil, fmt.Errorf("derivation missing output path for %q", name)
		}
		out[name] = p
	}
	return out, nil
}
