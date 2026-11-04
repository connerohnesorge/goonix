// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package builder executes Nix derivations against a local store (Phase 5).
//
// Builds run unsandboxed by default (see Config.Sandbox). Set Sandbox true only
// when a sandbox backend is wired; today it returns an error.
package builder

// Doc documents unsandboxed default for go doc consumers.
const Doc = "unsandboxed by default"
