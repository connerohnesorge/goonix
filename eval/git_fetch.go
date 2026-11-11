// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func gitExportTree(url, rev string) (checkoutDir string, cleanup func(), err error) {
	dir, cleanup, _, err := gitExportTreeMeta(url, rev, false, false)
	return dir, cleanup, err
}

func gitCloneURL(raw string) string {
	base, _, _ := strings.Cut(raw, "?")
	return base
}

func gitExportTreeMeta(url, rev string, withCount, submodules bool) (checkoutDir string, cleanup func(), meta treeMeta, err error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", nil, treeMeta{}, fmt.Errorf("git not found on PATH")
	}
	tmp, err := os.MkdirTemp("", "goonix-git-")
	if err != nil {
		return "", nil, treeMeta{}, err
	}
	cleanup = func() { os.RemoveAll(tmp) }
	repo := filepath.Join(tmp, "repo")
	if rev == "" {
		rev = "HEAD"
	}
	if err := runGit("", "clone", "--quiet", gitCloneURL(url), repo); err != nil {
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
	if submodules {
		if err := runGit(repo, "submodule", "update", "--init", "--recursive"); err != nil {
			cleanup()
			return "", nil, treeMeta{}, err
		}
	} else if err := removeGitlinks(repo); err != nil {
		// A gitlink with no checkout is an empty directory. Nix's export
		// omits it unless submodules=1.
		cleanup()
		return "", nil, treeMeta{}, err
	}
	if err := removeGitDirs(repo); err != nil {
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

func removeGitlinks(repo string) error {
	out, err := gitOutput(repo, "ls-files", "-s")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "160000 ") {
			continue
		}
		_, path, ok := strings.Cut(line, "\t")
		if !ok || path == "" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(repo, path)); err != nil {
			return err
		}
	}
	return nil
}

func removeGitDirs(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		if d.Name() != ".git" {
			return nil
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		if d.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
}

func runGit(dir string, args ...string) error {
	args = append([]string{"-c", "protocol.file.allow=always"}, args...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, out)
	}
	return nil
}
