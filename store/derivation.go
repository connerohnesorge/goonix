// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store

import (
	"io"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// Derivation is a parsed .drv file.
type Derivation = gonixdrv.Derivation

// ReadDerivation parses a derivation from r.
func ReadDerivation(r io.Reader) (*Derivation, error) {
	d, err := gonixdrv.ReadDerivation(r)
	if err != nil {
		return nil, err
	}
	ensureStructuredName(d)
	return d, nil
}

// DrvPath returns the store path for d using the default /nix/store prefix.
func DrvPath(d *Derivation) (string, error) {
	return DrvPathFromDerivation(Config{}, d)
}

// DrvPathWithConfig returns the store path for d, rewriting the prefix to cfg.
func DrvPathWithConfig(d *Derivation, cfg Config) (string, error) {
	return DrvPathFromDerivation(cfg, d)
}

// OutputPaths computes output store paths for d.
func OutputPaths(d *Derivation, inputReplacements map[string]string) (map[string]string, error) {
	return d.CalculateOutputPaths(inputReplacements)
}
