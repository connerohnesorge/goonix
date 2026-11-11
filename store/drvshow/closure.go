// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import (
	"fmt"
	"os"
	"sort"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// Loader loads a derivation by store .drv path.
type Loader func(drvPath string) (*gonixdrv.Derivation, error)

// LoadFromFile reads a .drv file from disk.
func LoadFromFile(drvPath string) (*gonixdrv.Derivation, error) {
	f, err := os.Open(drvPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return gonixdrv.ReadDerivation(f)
}

// BuildClosure walks inputDrvs recursively and returns a nix-show document.
func BuildClosure(rootDrvPath string, load Loader) (Document, error) {
	out := Document{
		Derivations: make(map[string]Entry),
		Version:     4,
	}
	seen := make(map[string]struct{})
	var walk func(string) error
	walk = func(drvPath string) error {
		if drvPath == "" {
			return nil
		}
		key := DrvKey(drvPath)
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		d, err := load(drvPath)
		if err != nil {
			return fmt.Errorf("load %q: %w", drvPath, err)
		}
		out.Derivations[key] = FromDerivation(d)
		paths := make([]string, 0, len(d.InputDerivations))
		for p := range d.InputDerivations {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if err := walk(p); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(rootDrvPath); err != nil {
		return Document{}, err
	}
	return out, nil
}
