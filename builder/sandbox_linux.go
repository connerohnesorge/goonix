// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (b *Builder) execBuildCommand(cmd *exec.Cmd) error {
	return b.execBuildCommandMount(cmd)
}

func absTool(name string) (string, error) {
	p, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	return filepath.Abs(p)
}

// execBuildCommandMount runs the builder in a user, mount, and pid namespace.
// Builds that are not fixed-output also enter a network namespace. The builder
// is uid 1000 and is pivoted into a root whose working directory is /build.
// The root contains that directory, the inputs' closure, /bin/sh and its
// loader, /dev basics, and /proc. errMountFallback is returned only when that
// setup does not start. The mount marker is a sibling of the host build
// directory and is removed after the build.
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
	exe, err := os.Executable()
	if err != nil {
		return errMountFallback
	}
	sentinel := cmd.Dir + ".goonix-mount-ok"
	_ = os.Remove(sentinel)
	defer os.Remove(sentinel)
	args := cmd.Args
	if len(args) == 0 {
		args = []string{cmd.Path}
	}
	spec := sandboxSpec{
		StorePhys: storePhys,
		BuildDir:  cmd.Dir,
		Sentinel:  sentinel,
		Builder:   cmd.Path,
		Args:      args,
		Env:       sandboxBuildEnv(cmd.Env, cmd.Dir),
		Closure:   append([]string(nil), b.plan.closure...),
		Outputs:   append([]string(nil), b.plan.outputs...),
	}
	f, err := os.CreateTemp("", "goonix-sandbox-spec-")
	if err != nil {
		return errMountFallback
	}
	specPath := f.Name()
	defer os.Remove(specPath)
	if err := json.NewEncoder(f).Encode(&spec); err != nil {
		f.Close()
		return errMountFallback
	}
	if err := f.Close(); err != nil {
		return errMountFallback
	}
	env := append(os.Environ(),
		"GOONIX_SANDBOX_STAGE=setup",
		"GOONIX_SANDBOX_SPEC="+specPath,
	)
	unshareArgs := []string{"-rmp", "--fork", "--propagation", "private"}
	if b.plan.isolateNet {
		unshareArgs = []string{"-n", "-rmp", "--fork", "--propagation", "private"}
	}
	unshareArgs = append(unshareArgs, exe)
	wrapped := exec.Command(unsharePath, unshareArgs...)
	wrapped.Env = env
	wrapped.Stdout = cmd.Stdout
	wrapped.Stderr = cmd.Stderr
	wrapped.Stdin = cmd.Stdin
	err = wrapped.Run()
	if err == nil {
		return nil
	}
	body, statErr := os.ReadFile(sentinel)
	if statErr != nil || string(body) != "ok\n" {
		return errMountFallback
	}
	return err
}

// sandboxBuildEnv points the builder at /build, which is where the host build
// directory is mounted inside the sandbox. The fallback path keeps the host
// directory because this rewrite is applied only to the sandboxed command.
func sandboxBuildEnv(env []string, hostDir string) []string {
	out := append([]string(nil), env...)
	for i, e := range out {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if v == hostDir {
			out[i] = k + "=/build"
			continue
		}
		if strings.HasPrefix(v, hostDir+"/") {
			out[i] = k + "=/build" + strings.TrimPrefix(v, hostDir)
		}
	}
	return out
}
