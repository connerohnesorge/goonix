// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package builder executes Nix derivations against a local store (Phase 5).
//
// On Linux, builds run in a user, mount, and pid namespace. Builds that are
// not fixed-output also get a network namespace, so they have no network.
// Fixed-output builds keep the network. The builder is uid 1000, its working
// directory is /build, and HOME is /homeless-shelter, which is not created.
// The root contains /build, the inputs' closure, /bin/sh, /dev basics, and
// /proc. When that setup cannot be created, the build fails unless
// AllowRelocated is set.
package builder

// Doc documents the Linux build sandbox for go doc consumers.
const Doc = "sandboxed by default on linux"
