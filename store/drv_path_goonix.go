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

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

// DrvPathFromDerivation returns the .drv store path for d (hashes full unmasked ATerm).
func DrvPathFromDerivation(cfg Config, d *gonixdrv.Derivation) (string, error) {
	if d == nil {
		return "", fmt.Errorf("nil derivation")
	}
	name := derivationName(d)
	if name == "" {
		return "", fmt.Errorf("derivation missing name")
	}
	h := sha256.New()
	if err := WriteDerivationForHash(h, d, false, nil); err != nil {
		return "", err
	}
	return drvPathFromATermDigest(cfg, h.Sum(nil), name, d)
}

// DrvPathFromATermBytes returns the store path for raw .drv ATerm bytes.
func DrvPathFromATermBytes(cfg Config, aterm []byte) (string, error) {
	d, err := gonixdrv.ReadDerivation(bytes.NewReader(aterm))
	if err != nil {
		return "", err
	}
	name := derivationName(d)
	if name == "" {
		return "", fmt.Errorf("derivation missing name")
	}
	h := sha256.New()
	h.Write(aterm)
	return drvPathFromATermDigest(cfg, h.Sum(nil), name, d)
}

func drvPathFromATermDigest(cfg Config, atermDigest []byte, name string, d *gonixdrv.Derivation) (string, error) {
	h := sha256.New()
	h.Write([]byte("text:"))
	// A DrvDeep context records the .drv as both an input derivation and an
	// input source. Nix's reference set keeps that path once.
	seen := make(map[string]struct{}, len(d.InputDerivations)+len(d.InputSources))
	refs := make([]string, 0, len(d.InputDerivations)+len(d.InputSources))
	add := func(p string) {
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		refs = append(refs, p)
	}
	for inputDrvPath := range d.InputDerivations {
		add(inputDrvPath)
	}
	for _, src := range d.InputSources {
		add(src)
	}
	sort.Strings(refs)
	for _, ref := range refs {
		h.Write([]byte(ref))
		h.Write([]byte(":"))
	}
	h.Write([]byte("sha256:"))
	encoded := make([]byte, hex.EncodedLen(sha256.Size))
	hex.Encode(encoded, atermDigest)
	h.Write(encoded)
	h.Write([]byte(":"))
	h.Write([]byte(DefaultStoreDir + ":"))
	h.Write([]byte(name))
	h.Write([]byte(".drv"))
	final := h.Sum(nil)
	sp := storepath.StorePath{
		Name:   name + ".drv",
		Digest: nixhash.CompressHash(final, 20),
	}
	return cfg.RewriteDefaultStoreDir(sp.Absolute()), nil
}
