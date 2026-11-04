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

// WithSearchPath prepends -I path entries (absolute directories).
func (np *NixPath) WithSearchPath(paths ...string) *NixPath {
	out := &NixPath{entries: append([]nixPathEntry{}, np.entries...)}
	for _, p := range paths {
		out.entries = append([]nixPathEntry{{path: p}}, out.entries...)
	}
	return out
}

// Entries returns string descriptions for builtins.nixPath.
func (np *NixPath) Entries() []string {
	out := make([]string, len(np.entries))
	for i, e := range np.entries {
		if e.prefix != "" {
			out[i] = e.prefix + "=" + e.path
		} else {
			out[i] = e.path
		}
	}
	return out
}

// FindFile resolves a logical name to an absolute filesystem path.
func (np *NixPath) FindFile(name string) (string, error) {
	for _, e := range np.entries {
		candidates := []string{
			filepath.Join(e.path, name),
			filepath.Join(e.path, name+".nix"),
		}
		if e.prefix != "" && e.prefix == name {
			if st, err := os.Stat(e.path); err == nil && st.IsDir() {
				return e.path, nil
			}
		}
		for _, c := range candidates {
			if st, err := os.Stat(c); err == nil && !st.IsDir() {
				return c, nil
			}
		}
	}
	return "", fmt.Errorf("file %q not found in NIX_PATH", name)
}

// ResolveAnglePath resolves <name> to a path string.
func (np *NixPath) ResolveAnglePath(lit string) (string, error) {
	if !strings.HasPrefix(lit, "<") || !strings.HasSuffix(lit, ">") {
		return lit, nil
	}
	name := strings.TrimSuffix(strings.TrimPrefix(lit, "<"), ">")
	return np.FindFile(name)
}
