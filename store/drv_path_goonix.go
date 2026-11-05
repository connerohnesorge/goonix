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
	refs := make([]string, 0, len(d.InputDerivations)+len(d.InputSources))
	for inputDrvPath := range d.InputDerivations {
		refs = append(refs, inputDrvPath)
	}
	refs = append(refs, d.InputSources...)
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
