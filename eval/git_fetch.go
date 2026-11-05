// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func gitExportTree(url, rev string) (checkoutDir string, cleanup func(), err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, fmt.Errorf("git not found on PATH")
	}
	tmp, err := os.MkdirTemp("", "goonix-git-")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(tmp) }
	repo := filepath.Join(tmp, "repo")
	if err := runGit("", "clone", "--quiet", url, repo); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := runGit(repo, "checkout", "-q", rev); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		cleanup()
		return "", nil, err
	}
	return repo, cleanup, nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
