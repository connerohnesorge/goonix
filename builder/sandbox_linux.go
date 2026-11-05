// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func (b *Builder) execBuildCommand(cmd *exec.Cmd) error {
	if err := b.execBuildCommandMount(cmd); err == nil {
		return nil
	}
	return errMountFallback
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\'\''`) + "'"
}

func (b *Builder) execBuildCommandMount(cmd *exec.Cmd) error {
	storePhys, err := filepath.Abs(filepath.Join(b.Store.Root, "store"))
	if err != nil {
		return err
	}
	inner := fmt.Sprintf(
		"set -e; mkdir -p /nix/store; mount --bind %q /nix/store; cd %q; exec %q",
		storePhys,
		cmd.Dir,
		cmd.Path,
	)
	args := make([]string, 0, len(cmd.Args))
	for _, a := range cmd.Args[1:] {
		args = append(args, shellQuote(a))
	}
	inner += " " + strings.Join(args, " ")
	wrapped := exec.Command("unshare", "-m", "--propagation", "private", "sh", "-c", inner)
	wrapped.Env = cmd.Env
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	wrapped.Stdin = cmd.Stdin
	return wrapped.Run()
}
