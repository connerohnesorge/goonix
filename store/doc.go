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

// Package store wraps github.com/nix-community/go-nix for pure store-path
// computation (hashes, NAR serialization, derivations, store paths).
//
// We reuse go-nix as a library for nixbase32, nixhash, nar, derivation, and
// storepath. SQLite-backed persistence in go-nix is intentionally not used;
// goonix will grow its own store interface later.
package store
