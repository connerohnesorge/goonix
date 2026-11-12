// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"

	"github.com/nix-community/go-nix/pkg/nixbase32"

	"github.com/connerohnesorge/goonix/store"
)

// NarHashAndSize computes the nixbase32 SHA-256 and byte length of path's NAR encoding.
func NarHashAndSize(path string) (hash string, size int64, err error) {
	var buf bytes.Buffer
	if err := store.DumpPath(&buf, path); err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(buf.Bytes())
	return nixbase32.EncodeToString(sum[:]), int64(buf.Len()), nil
}

// FlatNarHashAndSize hashes a single regular file as a flat NAR (root path "/").
func FlatNarHashAndSize(filePath string) (hash string, size int64, err error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", 0, err
	}
	if fi.IsDir() {
		return "", 0, os.ErrInvalid
	}
	return NarHashAndSize(filePath)
}

// HashNARStream returns nixbase32 SHA-256 and size for raw NAR bytes.
func HashNARStream(r io.Reader) (hash string, size int64, err error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	return nixbase32.EncodeToString(h.Sum(nil)), n, nil
}

// DumpPathToWriter writes NAR for path using go-nix.
func DumpPathToWriter(w io.Writer, path string) error {
	return store.DumpPath(w, path)
}
