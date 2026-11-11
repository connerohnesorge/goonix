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

package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/connerohnesorge/goonix/internal/nixfiles"
)

// NixPath resolves <name> paths using NIX_PATH and -I entries.
type NixPath struct {
	entries []nixPathEntry
}

type nixPathEntry struct {
	prefix string // empty means absolute path prefix in entry
	path   string
}

// NewNixPathFromEnv builds search path from NIX_PATH.
func NewNixPathFromEnv() *NixPath {
	raw := os.Getenv("NIX_PATH")
	return ParseNixPath(raw)
}

// ParseNixPath parses a NIX_PATH string (prefix=path components separated by ':').
func ParseNixPath(raw string) *NixPath {
	np := &NixPath{}
	if raw == "" {
		return np
	}
	for _, part := range strings.Split(raw, ":") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if idx := strings.Index(part, "="); idx >= 0 {
			np.entries = append(np.entries, nixPathEntry{
				prefix: part[:idx],
				path:   part[idx+1:],
			})
		} else {
			np.entries = append(np.entries, nixPathEntry{path: part})
		}
	}
	return np
}

// PrependEntries adds parsed NIX_PATH components before existing entries.
func (np *NixPath) PrependEntries(raw string) {
	merged := ParseNixPath(raw)
	np.entries = append(merged.entries, np.entries...)
}

// WithSearchPath prepends -I path entries (absolute directories).
func (np *NixPath) WithSearchPath(paths ...string) *NixPath {
	out := &NixPath{entries: append([]nixPathEntry{}, np.entries...)}
	for _, p := range paths {
		out.entries = append([]nixPathEntry{{path: p}}, out.entries...)
	}
	return out
}

// Entries returns the search path as (prefix, path) pairs, as builtins.nixPath
// shows them.
func (np *NixPath) Entries() [][2]string {
	out := make([][2]string, len(np.entries))
	for i, e := range np.entries {
		out[i] = [2]string{e.prefix, e.path}
	}
	return out
}

// FindFile resolves <name> against the search path, as Nix's
// EvalState::findFile does.
func (np *NixPath) FindFile(name string) (string, error) {
	return findInSearchPath(np.entries, name)
}

// findInSearchPath tries each entry in order. An entry with a prefix matches
// name itself or name below prefix/; an entry without a prefix is a
// directory that name is looked up in. The first existing candidate wins.
// Like Nix, a nix/... name that no entry provides falls back to the built-in
// corepkgs files (<nix/fetchurl.nix>).
func findInSearchPath(entries []nixPathEntry, name string) (string, error) {
	for _, e := range entries {
		var c string
		switch {
		case e.prefix == "":
			c = filepath.Join(e.path, name)
		case name == e.prefix:
			c = e.path
		case strings.HasPrefix(name, e.prefix+"/"):
			c = filepath.Join(e.path, name[len(e.prefix)+1:])
		default:
			continue
		}
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	if rest, ok := strings.CutPrefix(name, "nix/"); ok {
		if dir := corepkgsDir(); dir != "" {
			c := filepath.Join(dir, rest)
			if _, err := os.Stat(c); err == nil {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("file '%s' was not found in the Nix search path (add it using $NIX_PATH or -I)", name)
}

var corepkgsOnce sync.Once
var corepkgsPath string

// corepkgsDir is where the embedded <nix/...> files (fetchurl.nix and
// friends) are written on first use.
func corepkgsDir() string {
	corepkgsOnce.Do(func() {
		corepkgsPath, _ = nixfiles.MaterializedDir()
	})
	return corepkgsPath
}
