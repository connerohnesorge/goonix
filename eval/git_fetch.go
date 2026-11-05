// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func gitExportTree(url, rev string) (checkoutDir string, cleanup func(), err error) {
	dir, cleanup, _, err := gitExportTreeMeta(url, rev, false)
	return dir, cleanup, err
}

func gitExportTreeMeta(url, rev string, withCount bool) (checkoutDir string, cleanup func(), meta treeMeta, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, treeMeta{}, fmt.Errorf("git not found on PATH")
	}
	tmp, err := os.MkdirTemp("", "goonix-git-")
	if err != nil {
		return "", nil, treeMeta{}, err
	}
	cleanup = func() { os.RemoveAll(tmp) }
	repo := filepath.Join(tmp, "repo")
	if err := runGit("", "clone", "--quiet", url, repo); err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	if err := runGit(repo, "checkout", "-q", rev); err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	full, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	lm, err := gitOutput(repo, "log", "-1", "--format=%ct", "HEAD")
	if err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	unix, err := strconv.ParseInt(strings.TrimSpace(lm), 10, 64)
	if err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	meta.rev = strings.TrimSpace(full)
	meta.hasLast = true
	meta.lastModified = unix
	if withCount {
		countText, err := gitOutput(repo, "rev-list", "--count", "HEAD")
		if err != nil {
			cleanup()
			return "", nil, treeMeta{}, err
		}
		count, err := strconv.ParseInt(strings.TrimSpace(countText), 10, 64)
		if err != nil {
			cleanup()
			return "", nil, treeMeta{}, err
		}
		meta.revCount = &count
	}
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		cleanup()
		return "", nil, treeMeta{}, err
	}
	return repo, cleanup, meta, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
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
