// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Git object modes, matching Nix's git::Mode values.
const (
	gitModeDirectory  = 0o040000
	gitModeRegular    = 0o100644
	gitModeExecutable = 0o100755
	gitModeSymlink    = 0o120000
)

// GitObjectHash is Nix's git::dumpHash: the hex hash of the Git object at path.
// Directories hash as trees. Regular files and symlinks hash as blobs.
func GitObjectHash(path, algo string) (string, error) {
	sum, err := gitDumpHash(path, algo)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum), nil
}

// VerifyGitOutput checks a fixed-output git hash (Nix FileIngestionMethod::Git).
func VerifyGitOutput(path, wantBase16, algo string) error {
	if algo == "" || algo == "git:sha1" {
		algo = "sha1"
	}
	algo = trimGitAlgo(algo)
	if algo != "sha1" {
		return fmt.Errorf("Git file ingestion must use SHA-1 hash")
	}
	got, err := GitObjectHash(path, algo)
	if err != nil {
		return err
	}
	if got != wantBase16 {
		return fmt.Errorf("fixed-output hash mismatch: got %s want %s", got, wantBase16)
	}
	return nil
}

func trimGitAlgo(algo string) string {
	if len(algo) > 4 && algo[:4] == "git:" {
		return algo[4:]
	}
	return algo
}

func gitDumpHash(path, algo string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		return gitHashObject(algo, "blob", []byte(target))
	case fi.IsDir():
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		type treeEnt struct {
			sortName  string
			writeName string
			mode      uint32
			hash      []byte
		}
		var ents []treeEnt
		for _, e := range entries {
			child := filepath.Join(path, e.Name())
			sum, err := gitDumpHash(child, algo)
			if err != nil {
				return nil, err
			}
			mode, err := gitMode(child)
			if err != nil {
				return nil, err
			}
			sortName := e.Name()
			if mode == gitModeDirectory {
				sortName += "/"
			}
			ents = append(ents, treeEnt{sortName: sortName, writeName: e.Name(), mode: mode, hash: sum})
		}
		sort.Slice(ents, func(i, j int) bool { return ents[i].sortName < ents[j].sortName })
		var payload []byte
		for _, e := range ents {
			payload = append(payload, []byte(fmt.Sprintf("%o %s\x00", e.mode, e.writeName))...)
			payload = append(payload, e.hash...)
		}
		return gitHashObject(algo, "tree", payload)
	case fi.Mode().IsRegular():
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return gitHashObject(algo, "blob", data)
	default:
		return nil, fmt.Errorf("file %q is not a regular file, directory, or symlink", path)
	}
}

func gitMode(path string) (uint32, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		return gitModeSymlink, nil
	case fi.IsDir():
		return gitModeDirectory, nil
	case fi.Mode().IsRegular():
		if fi.Mode()&0o111 != 0 {
			return gitModeExecutable, nil
		}
		return gitModeRegular, nil
	default:
		return 0, fmt.Errorf("file %q is not a regular file, directory, or symlink", path)
	}
}

func gitHashObject(algo, typ string, payload []byte) ([]byte, error) {
	h, err := newGitHasher(algo)
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(h, fmt.Sprintf("%s %d\x00", typ, len(payload))); err != nil {
		return nil, err
	}
	if _, err := h.Write(payload); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func newGitHasher(algo string) (hash.Hash, error) {
	switch algo {
	case "md5":
		return md5.New(), nil
	case "sha1", "":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unknown hash algorithm '%s', expect 'md5', 'sha1', 'sha256', or 'sha512'", algo)
	}
}
