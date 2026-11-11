// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
)

// HashStringDigest returns the lowercase hex digest for Nix builtins.hashString.
func HashStringDigest(algo, s string) (string, error) {
	switch algo {
	case "md5":
		sum := md5.Sum([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	case "sha1":
		sum := sha1.Sum([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	case "sha256":
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	case "sha512":
		sum := sha512.Sum512([]byte(s))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("unknown hash type %q", algo)
	}
}
