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
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"
)

// TextHash returns the nixbase32 SHA-256 digest of s (text store object fingerprint).
func TextHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return nixbase32.EncodeToString(h[:])
}

// MakeStorePath returns an absolute store path for digest and name under cfg.
func (c Config) MakeStorePath(name string, digest []byte) (string, error) {
	return c.MakeAbsolute(name, digest)
}

// StorePathForText builds the store path for a text file with the given name and contents.
func (c Config) StorePathForText(name, contents string) (string, error) {
	contentHash := sha256.Sum256([]byte(contents))
	hexHash := hex.EncodeToString(contentHash[:])
	storeDir := c.storeDir()
	if storeDir == "" {
		storeDir = storepath.StoreDir
	}
	payload := "text:sha256:" + hexHash + ":" + storeDir + ":" + name
	digest := nixhash.CompressHash(sha256Sum([]byte(payload)), 20)
	return c.MakeStorePath(name, digest)
}

func sha256Sum(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// StorePathForFilesystemPath computes the store path for copying path into the store.
func (c Config) StorePathForFilesystemPath(sourcePath, name string) (string, error) {
	b32, err := RecursivePathHash(sourcePath)
	if err != nil {
		return "", err
	}
	raw, err := nixbase32.DecodeString(b32)
	if err != nil {
		return "", fmt.Errorf("decode path hash: %w", err)
	}
	base16 := hex.EncodeToString(raw)
	storeDir := c.storeDir()
	storeHash := hashStringsModulo("source", "sha256", base16, storeDir, name)
	digest := nixhash.CompressHash(storeHash, storepath.PathHashSize)
	return c.MakeStorePath(name, digest)
}

// WriteTextToStore writes contents to storePath (must already be computed).
func WriteTextToStore(storePath, contents string) error {
	if info, err := os.Stat(storePath); err == nil && info.IsDir() {
		if err := os.RemoveAll(storePath); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(storePath, []byte(contents), 0o444)
}

// CopyPathToStore copies a filesystem tree to storePath (caller must compute path first).
func CopyPathToStore(source, storePath string) error {
	return copyPathRecursive(source, storePath)
}

func copyPathRecursive(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.Symlink(target, dst)
	}
	if !info.IsDir() {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, info.Mode()&0o777); err != nil {
			return err
		}
		// A Windows executable mark lives in a stream the copy does not carry.
		if IsExecutable(src, info) {
			if dfi, err := os.Lstat(dst); err == nil && !IsExecutable(dst, dfi) {
				return MarkExecutable(dst)
			}
		}
		return nil
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyPathRecursive(src+"/"+e.Name(), dst+"/"+e.Name()); err != nil {
			return err
		}
	}
	return nil
}

// HashReader computes nixbase32 SHA-256 of all bytes read from r.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return nixbase32.EncodeToString(h.Sum(nil)), nil
}
