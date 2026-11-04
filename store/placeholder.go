// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"crypto/sha256"

	"github.com/nix-community/go-nix/pkg/nixbase32"
)

// OutputPlaceholder implements Nix libstore hashPlaceholder: "/" + nixBase32(compress(sha256("nix-output:" + name))).
func OutputPlaceholder(outputName string) string {
	sum := sha256.Sum256([]byte("nix-output:" + outputName))
	return "/" + nixbase32.EncodeToString(sum[:])
}
