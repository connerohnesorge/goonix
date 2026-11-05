// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (b *Builder) execBuildCommand(cmd *exec.Cmd) error {
	return b.execBuildCommandMount(cmd)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\'\''`) + "'"
}

// execBuildCommandMount runs the builder inside a mount namespace where the
// physical store is bind-mounted at /nix/store. It returns errMountFallback only
// when unshare or the bind mount itself fails. A nonzero builder exit is returned
// unchanged so the caller does not retry with physical paths.
func (b *Builder) execBuildCommandMount(cmd *exec.Cmd) error {
	if cmd.Dir == "" {
		return errMountFallback
	}
	storePhys, err := filepath.Abs(filepath.Join(b.Store.Root, "store"))
	if err != nil {
		return errMountFallback
	}
	sentinel := filepath.Join(cmd.Dir, ".goonix-mount-ok")
	_ = os.Remove(sentinel)

	inner := fmt.Sprintf(
		"set -e; mkdir -p /nix/store; mount --bind %s /nix/store; : > %s; cd %s; exec %s",
		shellQuote(storePhys),
		shellQuote(sentinel),
		shellQuote(cmd.Dir),
		shellQuote(cmd.Path),
	)
	for _, a := range cmd.Args[1:] {
		inner += " " + shellQuote(a)
	}
	wrapped := exec.Command("unshare", "-m", "--propagation", "private", "sh", "-c", inner)
	wrapped.Env = cmd.Env
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	wrapped.Stdin = cmd.Stdin
	err = wrapped.Run()
	if err == nil {
		return nil
	}
	if _, statErr := os.Stat(sentinel); statErr != nil {
		return errMountFallback
	}
	return err
}
