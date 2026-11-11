// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// AssertPinned fails the test if testdata/nixpkgs is not the pinned revision.
func (l Layout) AssertPinned(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(l.Nixpkgs); err != nil {
		t.Fatalf("nixpkgs checkout missing at %s: %v", l.Nixpkgs, err)
	}
	if rev := strings.TrimSpace(readFileIfExists(filepath.Join(l.Testdata, "pinned-nixpkgs-rev"))); rev != "" {
		if rev != NixpkgsRev {
			t.Fatalf("testdata/pinned-nixpkgs-rev=%q want %q", rev, NixpkgsRev)
		}
	} else if rev := strings.TrimSpace(readFileIfExists(filepath.Join(l.Nixpkgs, ".pinned-rev"))); rev != "" {
		if rev != NixpkgsRev {
			t.Fatalf("testdata/nixpkgs/.pinned-rev=%q want %q", rev, NixpkgsRev)
		}
	} else if gitRev := gitHeadRev(l.Nixpkgs); gitRev != "" {
		if gitRev != NixpkgsRev {
			t.Fatalf("nixpkgs git HEAD=%q want %q", gitRev, NixpkgsRev)
		}
	} else {
		t.Fatalf("nixpkgs pin not verified: add testdata/pinned-nixpkgs-rev or %s/.pinned-rev or a git checkout", l.Nixpkgs)
	}

	libVer, err := l.evalRaw(t, `(import <nixpkgs/lib>).version`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(libVer, "24.11") {
		t.Fatalf("lib.version=%q expected 24.11* (wrong nixpkgs on -I path?)", libVer)
	}
	rev, err := l.evalRaw(t, `(import <nixpkgs/lib>).trivial.revisionWithDefault ""`)
	if err != nil {
		t.Fatal(err)
	}
	if rev != "" && rev != NixpkgsRev && !strings.HasPrefix(NixpkgsRev, rev) {
		t.Fatalf("trivial.revisionWithDefault=%q want %q", rev, NixpkgsRev)
	}
}

func readFileIfExists(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func gitHeadRev(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return ""
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
