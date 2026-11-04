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
	"fmt"
	"path"
	"strings"

	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/storepath"
)

// DefaultStoreDir is the conventional Nix store root.
const DefaultStoreDir = storepath.StoreDir

// Config selects the store directory prefix for computed paths.
type Config struct {
	StoreDir string
}

func (c Config) storeDir() string {
	if c.StoreDir == "" {
		return DefaultStoreDir
	}
	return c.StoreDir
}

// ParseRelative parses the `hash-name` portion of a store path.
func ParseRelative(s string) (*storepath.StorePath, error) {
	return storepath.FromString(s)
}

// ParseAbsolute parses a full store path, validating the configured prefix.
func (c Config) ParseAbsolute(abs string) (*storepath.StorePath, error) {
	dir := c.storeDir()
	prefix := dir + "/"
	if !strings.HasPrefix(abs, prefix) {
		return nil, fmt.Errorf("store path %q does not start with %q", abs, prefix)
	}
	return storepath.FromString(abs[len(prefix):])
}

// Absolute renders sp under the configured store directory.
func (c Config) Absolute(sp *storepath.StorePath) string {
	return path.Join(c.storeDir(), sp.String())
}

// MakeAbsolute builds a store path from a compressed digest and name.
func (c Config) MakeAbsolute(name string, digest []byte) (string, error) {
	sp := storepath.StorePath{Name: name, Digest: digest}
	if err := sp.Validate(); err != nil {
		return "", err
	}
	return c.Absolute(&sp), nil
}

// RewriteDefaultStoreDir replaces the go-nix default prefix with cfg's store dir.
func (c Config) RewriteDefaultStoreDir(p string) string {
	if c.storeDir() == DefaultStoreDir {
		return p
	}
	if strings.HasPrefix(p, DefaultStoreDir+"/") {
		return c.storeDir() + p[len(DefaultStoreDir):]
	}
	return p
}

// EncodeDigest renders digest as nixbase32 (used in store path file names).
func EncodeDigest(digest []byte) string {
	return nixbase32.EncodeToString(digest)
}
