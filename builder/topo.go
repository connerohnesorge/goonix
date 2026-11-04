// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"
	"sort"
)

// TopoSort returns derivations in build order (inputs before dependents).
func TopoSort(root string, load Loader) ([]string, error) {
	seen := map[string]struct{}{}
	order := []string{}
	var walk func(string) error
	walk = func(drvPath string) error {
		if drvPath == "" {
			return nil
		}
		if _, ok := seen[drvPath]; ok {
			return nil
		}
		d, err := load(drvPath)
		if err != nil {
			return err
		}
		inputs := make([]string, 0, len(d.InputDerivations))
		for p := range d.InputDerivations {
			inputs = append(inputs, p)
		}
		sort.Strings(inputs)
		for _, p := range inputs {
			if err := walk(p); err != nil {
				return err
			}
		}
		seen[drvPath] = struct{}{}
		order = append(order, drvPath)
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return order, nil
}

// ValidateTopo ensures the order respects edges (test helper).
func ValidateTopo(order []string, load Loader) error {
	pos := make(map[string]int, len(order))
	for i, p := range order {
		pos[p] = i
	}
	for i, p := range order {
		d, err := load(p)
		if err != nil {
			return err
		}
		for inp := range d.InputDerivations {
			j, ok := pos[inp]
			if !ok {
				return fmt.Errorf("missing input %s for %s", inp, p)
			}
			if j >= i {
				return fmt.Errorf("input %s not before %s", inp, p)
			}
		}
	}
	return nil
}

