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
	"crypto/sha256"
	"io"

	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

// ParseHash parses a Nix hash string (hex, nix base32, SRI, etc.).
func ParseHash(s string) (*nixhash.HashWithEncoding, error) {
	return nixhash.ParseAny(s, nil)
}

// FormatHash renders h with the requested encoding.
func FormatHash(h *nixhash.Hash, enc nixhash.Encoding, includeAlgo bool) string {
	return h.Format(enc, includeAlgo)
}

// CompressHash shortens a digest the way Nix store paths do.
func CompressHash(input []byte, size int) []byte {
	return nixhash.CompressHash(input, size)
}

// RecursivePathHash returns the nixbase32 SHA-256 hash of path's NAR representation.
func RecursivePathHash(path string) (string, error) {
	h := sha256.New()
	if err := dumpNAR(h, path, nil); err != nil {
		return "", err
	}
	return nixbase32.EncodeToString(h.Sum(nil)), nil
}

// DumpPath writes the NAR encoding of path to w.
func DumpPath(w io.Writer, path string) error {
	return dumpNAR(w, path, nil)
}
