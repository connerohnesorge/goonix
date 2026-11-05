// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package builder

import (
	"os/exec"
)

func (b *Builder) execBuildCommand(cmd *exec.Cmd) error {
	return errMountFallback
}
