// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

// ModuloKind matches Nix DrvHash::Kind for output-path purposes.
type ModuloKind int

const (
	ModuloRegular ModuloKind = iota
	ModuloDeferred
)

// ModuloResult is the output of hashDerivationModulo (Nix DrvHash).
type ModuloResult struct {
	Kind   ModuloKind
	Hashes map[string][]byte // per output name, raw SHA-256 digest
}

// ModuloCache memoizes pathDerivationModulo by derivation store path.
type ModuloCache struct {
	byPath map[string]*gonixdrv.Derivation
	modulo map[string]ModuloResult
}

// NewModuloCache returns an empty derivation modulo cache.
func NewModuloCache() *ModuloCache {
	return &ModuloCache{
		byPath: make(map[string]*gonixdrv.Derivation),
		modulo: make(map[string]ModuloResult),
	}
}

// Register stores a derivation for later lookup by its drvPath.
func (c *ModuloCache) Register(drvPath string, d *gonixdrv.Derivation) {
	if c == nil || drvPath == "" || d == nil {
		return
	}
	if c.byPath == nil {
		c.byPath = make(map[string]*gonixdrv.Derivation)
	}
	c.byPath[drvPath] = d
}

// Lookup returns a registered derivation by store path.
func (c *ModuloCache) Lookup(drvPath string) (*gonixdrv.Derivation, bool) {
	if c == nil {
		return nil, false
	}
	d, ok := c.byPath[drvPath]
	return d, ok
}

// HashDerivationModulo implements Nix 2.24.9 hashDerivationModulo semantics.
// When maskOutputs is true, output paths and matching env vars are blanked (static output hashes).
// Input derivations are replaced with hex keys from pathDerivationModulo (maskOutputs=false on deps).
func (c *ModuloCache) HashDerivationModulo(d *gonixdrv.Derivation, maskOutputs bool) (ModuloResult, error) {
	if d == nil {
		return ModuloResult{}, fmt.Errorf("nil derivation")
	}
	if fixed, ok := fixedOutputModulo(d); ok {
		return fixed, nil
	}
	reps, kind, err := c.buildInputReplacements(d)
	if err != nil {
		return ModuloResult{}, err
	}
	digest, err := hashDerivationATerm(d, maskOutputs, reps)
	if err != nil {
		return ModuloResult{}, err
	}
	hashes := make(map[string][]byte, len(d.Outputs))
	for outName := range d.Outputs {
		hashes[outName] = digest
	}
	return ModuloResult{Kind: kind, Hashes: hashes}, nil
}

// PathDerivationModulo is hashDerivationModulo(dep, maskOutputs=false) memoized by drvPath.
func (c *ModuloCache) PathDerivationModulo(drvPath string) (ModuloResult, error) {
	if c == nil {
		return ModuloResult{}, fmt.Errorf("nil modulo cache")
	}
	if r, ok := c.modulo[drvPath]; ok {
		return r, nil
	}
	d, ok := c.byPath[drvPath]
	if !ok {
		return ModuloResult{}, fmt.Errorf("derivation %q not in modulo cache", drvPath)
	}
	r, err := c.HashDerivationModulo(d, false)
	if err != nil {
		return ModuloResult{}, err
	}
	c.modulo[drvPath] = r
	return r, nil
}

func (c *ModuloCache) buildInputReplacements(d *gonixdrv.Derivation) (inputReplacements, ModuloKind, error) {
	reps := make(inputReplacements, len(d.InputDerivations))
	kind := ModuloRegular
	paths := make([]string, 0, len(d.InputDerivations))
	for p := range d.InputDerivations {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, drvPath := range paths {
		depRes, err := c.PathDerivationModulo(drvPath)
		if err != nil {
			return nil, kind, err
		}
		if depRes.Kind == ModuloDeferred {
			kind = ModuloDeferred
		}
		outNames := d.InputDerivations[drvPath]
		if len(outNames) == 0 {
			return nil, kind, fmt.Errorf("empty input outputs for %q", drvPath)
		}
		h, ok := depRes.Hashes[outNames[0]]
		if !ok {
			return nil, kind, fmt.Errorf("no modulo hash for output %q of %q", outNames[0], drvPath)
		}
		for _, o := range outNames[1:] {
			h2, ok := depRes.Hashes[o]
			if !ok || !bytes.Equal(h, h2) {
				return nil, kind, fmt.Errorf("inconsistent modulo hashes for %q outputs", drvPath)
			}
		}
		reps[drvPath] = hex.EncodeToString(h)
	}
	return reps, kind, nil
}

func hashDerivationATerm(d *gonixdrv.Derivation, maskOutputs bool, reps inputReplacements) ([]byte, error) {
	h := sha256.New()
	if err := writeDerivationForHash(h, d, maskOutputs, reps); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func fixedOutputModulo(d *gonixdrv.Derivation) (ModuloResult, bool) {
	if len(d.Outputs) == 0 {
		return ModuloResult{}, false
	}
	hashes := make(map[string][]byte, len(d.Outputs))
	allFixed := true
	for outName, o := range d.Outputs {
		if o.HashAlgorithm == "" || o.Hash == "" {
			allFixed = false
			break
		}
		outPath := o.Path
		if outPath == "" {
			outPath = d.Env[outName]
		}
		if outPath == "" {
			allFixed = false
			break
		}
		methodAlgo := fixedMethodAlgo(d, o)
		payload := "fixed:out:" + methodAlgo + ":" + o.Hash + ":" + outPath
		sum := sha256.Sum256([]byte(payload))
		hashes[outName] = sum[:]
	}
	if !allFixed {
		return ModuloResult{}, false
	}
	return ModuloResult{Kind: ModuloRegular, Hashes: hashes}, true
}

func fixedMethodAlgo(d *gonixdrv.Derivation, o *gonixdrv.Output) string {
	algo := o.HashAlgorithm
	if algo == "" {
		algo = "sha256"
	}
	if strings.HasPrefix(algo, "r:") {
		return algo
	}
	mode := d.Env["outputHashMode"]
	switch mode {
	case "nar", "recursive":
		return "r:" + strings.TrimPrefix(algo, "r:")
	case "git":
		return "git:" + strings.TrimPrefix(algo, "git:")
	default:
		// flat (default): Nix renderPrefix is empty, printMethodAlgo is just printHashAlgo
		return strings.TrimPrefix(algo, "r:")
	}
}

// OutputPathsModulo computes output store paths using hashDerivationModulo (maskOutputs=true).
func OutputPathsModulo(cfg Config, d *gonixdrv.Derivation, cache *ModuloCache) (map[string]string, error) {
	name := d.Name()
	if name == "" {
		return nil, fmt.Errorf("derivation missing name")
	}
	res, err := cache.HashDerivationModulo(d, true)
	if err != nil {
		return nil, err
	}
	if res.Kind == ModuloDeferred {
		return nil, fmt.Errorf("deferred output paths not supported")
	}
	out := make(map[string]string, len(d.Outputs))
	storeDir := cfg.storeDir()
	for outName := range d.Outputs {
		pathName := name
		if outName != "out" {
			pathName = name + "-" + outName
		}
		digest, ok := res.Hashes[outName]
		if !ok {
			return nil, fmt.Errorf("missing modulo hash for output %q", outName)
		}
		storeHash := hashStringsModulo("output", outName, "sha256", hex.EncodeToString(digest), storeDir, pathName)
		sp := storepath.StorePath{
			Name:   pathName,
			Digest: nixhash.CompressHash(storeHash, storepath.PathHashSize),
		}
		out[outName] = cfg.RewriteDefaultStoreDir(sp.Absolute())
	}
	return out, nil
}

func hashStringsModulo(parts ...string) []byte {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{':'})
		}
		h.Write([]byte(p))
	}
	return h.Sum(nil)
}

// InputReplacementsHex builds the map expected by go-nix CalculateOutputPaths (Nix inputs2 keys).
func InputReplacementsHex(cache *ModuloCache, d *gonixdrv.Derivation) (map[string]string, error) {
	reps, _, err := cache.buildInputReplacements(d)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(reps))
	for k, v := range reps {
		out[k] = v
	}
	return out, nil
}

// DrvPathFromDerivation returns the .drv store path for d (hashes full unmasked ATerm).
func DrvPathFromDerivation(cfg Config, d *gonixdrv.Derivation) (string, error) {
	return d.DrvPath()
}
