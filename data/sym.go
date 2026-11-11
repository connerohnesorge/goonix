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

package data

import "sync"

// Symbol is an interned attribute name or identifier.
type Symbol uint32

// InvalidSymbol is returned for unknown lookups.
const InvalidSymbol Symbol = 0

// Interner deduplicates strings into compact Symbol values.
type Interner struct {
	mu      sync.RWMutex
	strings []string
	index   map[string]Symbol
}

// NewInterner returns an empty intern table.
func NewInterner() *Interner {
	return &Interner{
		strings: []string{""},
		index:   map[string]Symbol{"": InvalidSymbol},
	}
}

// Intern returns a stable Symbol for s, allocating on first use.
func (in *Interner) Intern(s string) Symbol {
	in.mu.RLock()
	if sym, ok := in.index[s]; ok {
		in.mu.RUnlock()
		return sym
	}
	in.mu.RUnlock()

	in.mu.Lock()
	defer in.mu.Unlock()
	if sym, ok := in.index[s]; ok {
		return sym
	}
	sym := Symbol(len(in.strings))
	in.strings = append(in.strings, s)
	in.index[s] = sym
	return sym
}

// Lookup returns the string for sym, or "" if unknown.
func (in *Interner) Lookup(sym Symbol) string {
	in.mu.RLock()
	defer in.mu.RUnlock()
	if int(sym) >= len(in.strings) {
		return ""
	}
	return in.strings[sym]
}
