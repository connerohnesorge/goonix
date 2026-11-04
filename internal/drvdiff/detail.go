// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvdiff

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/store/drvshow"
)

const storePrefix = "/nix/store/"

// FormatMismatchDetail prints field-level comparison plus raw .drv ATerm for both sides.
func FormatMismatchDetail(r *Result, nixEntry, goEntry drvshow.Entry, nixATerm, goATerm []byte) string {
	if r == nil {
		return FormatResult(nil)
	}
	var b strings.Builder
	b.WriteString(FormatResult(r))
	b.WriteString("\n--- field dump (nix vs goonix) ---\n")
	fmt.Fprintf(&b, "builder:\n  nix:    %s\n  goonix: %s\n", nixEntry.Builder, goEntry.Builder)
	fmt.Fprintf(&b, "system:\n  nix:    %s\n  goonix: %s\n", nixEntry.System, goEntry.System)
	fmt.Fprintf(&b, "name:\n  nix:    %s\n  goonix: %s\n", nixEntry.Name, goEntry.Name)
	fmt.Fprintf(&b, "args:\n  nix:    %s\n  goonix: %s\n", fmt.Sprint(nixEntry.Args), fmt.Sprint(goEntry.Args))
	dumpEnv(&b, nixEntry.Env, goEntry.Env)
	dumpOutputs(&b, nixEntry.Outputs, goEntry.Outputs)
	fmt.Fprintf(&b, "inputs.srcs:\n  nix:    %s\n  goonix: %s\n", fmt.Sprint(nixEntry.Inputs.Srcs), fmt.Sprint(goEntry.Inputs.Srcs))
	fmt.Fprintf(&b, "inputs.drvs:\n  nix:    %s\n  goonix: %s\n", fmt.Sprint(normalizeInputDrvs(nixEntry.Inputs.Drvs)), fmt.Sprint(normalizeInputDrvs(goEntry.Inputs.Drvs)))
	b.WriteString("\n--- raw ATerm ---\n")
	fmt.Fprintf(&b, "nix:\n%s\n", nixATerm)
	fmt.Fprintf(&b, "goonix:\n%s\n", goATerm)
	return b.String()
}

func dumpEnv(b *strings.Builder, n, g map[string]string) {
	keys := map[string]struct{}{}
	for k := range n {
		keys[k] = struct{}{}
	}
	for k := range g {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	b.WriteString("env:\n")
	for _, k := range sorted {
		fmt.Fprintf(b, "  %s:\n    nix:    %q\n    goonix: %q\n", k, n[k], g[k])
	}
}

func dumpOutputs(b *strings.Builder, n, g map[string]drvshow.Output) {
	keys := map[string]struct{}{}
	for k := range n {
		keys[k] = struct{}{}
	}
	for k := range g {
		keys[k] = struct{}{}
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	b.WriteString("outputs:\n")
	for _, k := range sorted {
		no, goo := n[k], g[k]
		fmt.Fprintf(b, "  %s.path:\n    nix:    %q\n    goonix: %q\n", k, no.Path, goo.Path)
		fmt.Fprintf(b, "  %s.hashAlgo:\n    nix:    %q\n    goonix: %q\n", k, no.HashAlgo, goo.HashAlgo)
		fmt.Fprintf(b, "  %s.hash:\n    nix:    %q\n    goonix: %q\n", k, no.Hash, goo.Hash)
	}
}

// ReadNixStoreATerm loads /nix/store/<drvKey> when present.
func ReadNixStoreATerm(drvKey string) []byte {
	if drvKey == "" {
		return nil
	}
	p := filepath.Join(storePrefix, drvKey)
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return b
}
