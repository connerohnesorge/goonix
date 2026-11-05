// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

// FixedOutputPaths computes store paths for fixed-output derivations (Nix 2.24.9 makeFixedOutputPath).
func FixedOutputPaths(cfg Config, d *gonixdrv.Derivation) (map[string]string, error) {
	if d == nil {
		return nil, fmt.Errorf("nil derivation")
	}
	name := derivationName(d)
	if name == "" {
		return nil, fmt.Errorf("derivation missing name")
	}
	out := make(map[string]string, len(d.Outputs))
	for outName, o := range d.Outputs {
		if o == nil || o.Hash == "" {
			return nil, fmt.Errorf("output %q is not fixed", outName)
		}
		p, err := MakeFixedOutputPath(cfg, d, outName, o)
		if err != nil {
			return nil, err
		}
		out[outName] = p
	}
	return out, nil
}

// MakeFixedOutputPath implements Nix 2.24.9 store-api makeFixedOutputPath.
func MakeFixedOutputPath(cfg Config, d *gonixdrv.Derivation, outName string, o *gonixdrv.Output) (string, error) {
	pathName := SanitizeStorePathName(d.Name())
	if outName != "out" {
		pathName = SanitizeStorePathName(d.Name() + "-" + outName)
	}
	algo := fixedMethodAlgo(d, o)
	base16Hash := o.Hash

	if o.HashAlgorithm == "r:sha256" || algo == "r:sha256" {
		storeHash := hashStringsModulo("source", "sha256", base16Hash, cfg.storeDir(), pathName)
		return makeStorePathFromHash(cfg, pathName, storeHash)
	}

	methodAlgo := algo
	innerPayload := "fixed:out:" + methodAlgo + ":" + base16Hash + ":"
	sum := sha256.Sum256([]byte(innerPayload))
	storeHash := hashStringsModulo("output", "out", "sha256", hex.EncodeToString(sum[:]), cfg.storeDir(), pathName)
	return makeStorePathFromHash(cfg, pathName, storeHash)
}

func makeStorePathFromHash(cfg Config, pathName string, storeHash []byte) (string, error) {
	sp := storepath.StorePath{
		Name:   pathName,
		Digest: nixhash.CompressHash(storeHash, storepath.PathHashSize),
	}
	if err := sp.Validate(); err != nil {
		return "", err
	}
	return cfg.RewriteDefaultStoreDir(sp.Absolute()), nil
}

// fixedOutputPrintAlgo is ContentAddress::printMethodAlgo for the fixed-output inner hash.
func fixedMethodAlgo(d *gonixdrv.Derivation, o *gonixdrv.Output) string {
	algo := o.HashAlgorithm
	if algo == "" {
		algo = "sha256"
	}
	if strings.HasPrefix(algo, "r:") || strings.HasPrefix(algo, "git:") {
		return algo
	}
	mode := d.Env["outputHashMode"]
	switch mode {
	case "nar", "recursive":
		return "r:" + strings.TrimPrefix(algo, "r:")
	case "git":
		return "git:" + strings.TrimPrefix(algo, "git:")
	default:
		return strings.TrimPrefix(algo, "r:")
	}
}

// FixedOutputDrvHashAlgo is the hashAlgo field stored in the .drv output entry.
func FixedOutputDrvHashAlgo(d *gonixdrv.Derivation, o *gonixdrv.Output) string {
	return fixedMethodAlgo(d, o)
}

// ParseFixedOutputHash resolves env outputHash / outputHashAlgo into base16 hash and drv hashAlgo.
func ParseFixedOutputHash(envHash, envAlgo, envMode string) (base16 string, drvAlgo string, err error) {
	if envHash == "" {
		return "", "", fmt.Errorf("missing outputHash")
	}
	hw, err := ParseHash(envHash)
	if err != nil {
		// Nix keeps legacy nix-base32 in env when outputHashAlgo is explicit (fetchurl in nixpkgs).
		if envAlgo == "" {
			return "", "", err
		}
		algo, aerr := nixhash.ParseAlgorithm(envAlgo)
		if aerr != nil {
			return "", "", err
		}
		hw, err = nixhash.ParseAny(envHash, &algo)
		if err != nil {
			return "", "", err
		}
	}
	base16 = hw.Hash.Format(nixhash.Base16, false)
	algo := envAlgo
	if algo == "" {
		algo = hw.Hash.Algo().String()
	}
	mode := envMode
	switch mode {
	case "nar", "recursive":
		drvAlgo = "r:" + strings.TrimPrefix(algo, "r:")
	case "git":
		drvAlgo = "git:" + strings.TrimPrefix(algo, "git:")
	default:
		drvAlgo = strings.TrimPrefix(algo, "r:")
	}
	return base16, drvAlgo, nil
}
