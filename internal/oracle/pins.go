// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import "github.com/connerohnesorge/goonix/internal/nixversion"

// Pinned upstream pair for goldens and parity tests.
const (
	NixVersion     = nixversion.Version
	NixpkgsRev     = "50ab793786d9de88ee30ec4e4c24fb4236fc2674"
	NixpkgsTarball = "https://github.com/NixOS/nixpkgs/archive/" + NixpkgsRev + ".tar.gz"
)
