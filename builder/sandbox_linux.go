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

func absTool(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// execBuildCommandMount runs the builder inside a user and mount namespace where
// the physical store is bind-mounted at /nix/store. The wrapper resolves mkdir,
// mount, and sh by absolute path so it does not depend on the builder environment.
// The builder process still receives cmd.Env unchanged (PATH=/path-not-set).
// errMountFallback is returned only when unshare or the bind mount fails.
func (b *Builder) execBuildCommandMount(cmd *exec.Cmd) error {
	if cmd.Dir == "" {
		return errMountFallback
	}
	storePhys, err := filepath.Abs(filepath.Join(b.Store.Root, "store"))
	if err != nil {
		return errMountFallback
	}
	unsharePath, err := absTool("unshare")
	if err != nil {
		return errMountFallback
	}
	shPath, err := absTool("sh")
	if err != nil {
		return errMountFallback
	}
	mkdirPath, err := absTool("mkdir")
	if err != nil {
		return errMountFallback
	}
	mountPath, err := absTool("mount")
	if err != nil {
		return errMountFallback
	}
	sentinel := filepath.Join(cmd.Dir, ".goonix-mount-ok")
	_ = os.Remove(sentinel)

	// Absolute paths: the shell inherits PATH=/path-not-set from the builder env.
	inner := fmt.Sprintf(
		"set -e; %s -p /nix/store; %s --bind %s /nix/store; : > %s; cd %s; exec %s",
		mkdirPath,
		mountPath,
		shellQuote(storePhys),
		shellQuote(sentinel),
		shellQuote(cmd.Dir),
		shellQuote(cmd.Path),
	)
	for _, a := range cmd.Args[1:] {
		inner += " " + shellQuote(a)
	}
	wrapped := exec.Command(unsharePath, "-rm", "--propagation", "private", shPath, "-c", inner)
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
