// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvdiff

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/store/drvshow"
)

// FieldDiff is one differing scalar or map entry.
type FieldDiff struct {
	Path   string
	Nix    string
	Goonix string
}

// Result is the deepest mismatch where all inputs agree.
type Result struct {
	NixKey     string
	GoonixKey  string
	LogicalName string
	Depth      int
	Diffs      []FieldDiff
}

// Compare finds the deepest derivation whose own fields differ while matched inputs agree.
func Compare(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string) (*Result, error) {
	if _, ok := nixDoc.Derivations[nixRootKey]; !ok {
		return nil, fmt.Errorf("nix root %q not in closure", nixRootKey)
	}
	if _, ok := goDoc.Derivations[goRootKey]; !ok {
		return nil, fmt.Errorf("goonix root %q not in closure", goRootKey)
	}
	var best *Result
	seen := make(map[[2]string]struct{})
	var visit func(nixKey, goKey string, depth int)
	visit = func(nixKey, goKey string, depth int) {
		if nixKey == "" || goKey == "" {
			return
		}
		pair := [2]string{nixKey, goKey}
		if _, ok := seen[pair]; ok {
			return
		}
		seen[pair] = struct{}{}
		n := nixDoc.Derivations[nixKey]
		g := goDoc.Derivations[goKey]
		pairs := pairInputDrvs(n, g)
		for _, p := range pairs {
			if p.nixKey != "" && p.goKey != "" && p.nixKey != p.goKey {
				visit(p.nixKey, p.goKey, depth+1)
			}
		}
		diffs := fieldDiffs(n, g)
		if len(diffs) == 0 {
			return
		}
		if !inputsRecursivelyEqual(nixDoc, goDoc, n, g, seen) {
			return
		}
		r := &Result{
			NixKey:      nixKey,
			GoonixKey:   goKey,
			LogicalName: drvshow.LogicalName(nixKey),
			Depth:       depth,
			Diffs:       diffs,
		}
		if best == nil || r.Depth >= best.Depth {
			best = r
		}
	}
	visit(nixRootKey, goRootKey, 0)
	return best, nil
}

type drvPair struct {
	nixKey string
	goKey  string
	logical string
}

func pairInputDrvs(n, g drvshow.Entry) []drvPair {
	goByLogical := make(map[string]string, len(g.Inputs.Drvs))
	for k := range g.Inputs.Drvs {
		goByLogical[drvshow.LogicalName(k)] = k
	}
	var pairs []drvPair
	for nixKey := range n.Inputs.Drvs {
		logical := drvshow.LogicalName(nixKey)
		goKey := goByLogical[logical]
		if goKey == "" {
			pairs = append(pairs, drvPair{nixKey: nixKey, goKey: "", logical: logical})
			continue
		}
		pairs = append(pairs, drvPair{nixKey: nixKey, goKey: goKey, logical: logical})
		delete(goByLogical, logical)
	}
	for logical, goKey := range goByLogical {
		pairs = append(pairs, drvPair{nixKey: "", goKey: goKey, logical: logical})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].logical < pairs[j].logical })
	return pairs
}

func inputsRecursivelyEqual(nixDoc, goDoc drvshow.Document, n, g drvshow.Entry, seen map[[2]string]struct{}) bool {
	for _, p := range pairInputDrvs(n, g) {
		if p.nixKey == "" || p.goKey == "" {
			return false
		}
		pair := [2]string{p.nixKey, p.goKey}
		if _, ok := seen[pair]; ok {
			continue
		}
		ne := nixDoc.Derivations[p.nixKey]
		ge := goDoc.Derivations[p.goKey]
		if len(fieldDiffs(ne, ge)) > 0 {
			return false
		}
		if !inputsRecursivelyEqual(nixDoc, goDoc, ne, ge, seen) {
			return false
		}
	}
	return true
}

func fieldDiffs(n, g drvshow.Entry) []FieldDiff {
	var out []FieldDiff
	if n.Builder != g.Builder {
		out = append(out, FieldDiff{"builder", n.Builder, g.Builder})
	}
	if n.System != g.System {
		out = append(out, FieldDiff{"system", n.System, g.System})
	}
	if n.Name != g.Name {
		out = append(out, FieldDiff{"name", n.Name, g.Name})
	}
	if !reflect.DeepEqual(normalizeStringList(n.Args), normalizeStringList(g.Args)) {
		out = append(out, FieldDiff{"args", fmt.Sprint(n.Args), fmt.Sprint(g.Args)})
	}
	out = append(out, envDiffs(n.Env, g.Env)...)
	out = append(out, outputDiffs(n.Outputs, g.Outputs)...)
	out = append(out, inputSrcDiffs(n.Inputs.Srcs, g.Inputs.Srcs)...)
	out = append(out, inputDrvDiffs(n.Inputs.Drvs, g.Inputs.Drvs)...)
	return out
}

func envDiffs(n, g map[string]string) []FieldDiff {
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
	var out []FieldDiff
	for _, k := range sorted {
		nv, gv := drvshow.NormalizePathValue(n[k]), drvshow.NormalizePathValue(g[k])
		if nv == gv {
			continue
		}
		out = append(out, FieldDiff{Path: "env." + k, Nix: n[k], Goonix: g[k]})
	}
	return out
}

func outputDiffs(n, g map[string]drvshow.Output) []FieldDiff {
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
	var out []FieldDiff
	for _, k := range sorted {
		nv, gv := drvshow.NormalizePathValue(n[k].Path), drvshow.NormalizePathValue(g[k].Path)
		if nv != gv {
			out = append(out, FieldDiff{Path: "outputs." + k + ".path", Nix: n[k].Path, Goonix: g[k].Path})
		}
		nha, gha := n[k].HashAlgo, g[k].HashAlgo
		if nha != gha {
			out = append(out, FieldDiff{Path: "outputs." + k + ".hashAlgo", Nix: nha, Goonix: gha})
		}
		nh, gh := n[k].Hash, g[k].Hash
		if nh != gh {
			out = append(out, FieldDiff{Path: "outputs." + k + ".hash", Nix: nh, Goonix: gh})
		}
	}
	return out
}

func normalizeStringList(ss []string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := append([]string(nil), ss...)
	for i, s := range out {
		out[i] = drvshow.NormalizePathValue(s)
	}
	return out
}

func inputSrcDiffs(n, g []string) []FieldDiff {
	if reflect.DeepEqual(normalizeStringList(n), normalizeStringList(g)) {
		return nil
	}
	return []FieldDiff{{Path: "inputs.srcs", Nix: fmt.Sprint(n), Goonix: fmt.Sprint(g)}}
}

func inputDrvDiffs(n, g map[string]drvshow.InputDrv) []FieldDiff {
	nNorm := normalizeInputDrvs(n)
	gNorm := normalizeInputDrvs(g)
	if reflect.DeepEqual(nNorm, gNorm) {
		return nil
	}
	return []FieldDiff{{Path: "inputs.drvs (by logical name)", Nix: fmt.Sprint(nNorm), Goonix: fmt.Sprint(gNorm)}}
}

func normalizeInputDrvs(m map[string]drvshow.InputDrv) map[string]drvshow.InputDrv {
	out := make(map[string]drvshow.InputDrv, len(m))
	for k, v := range m {
		out[drvshow.LogicalName(k)] = v
	}
	return out
}

// FormatResult prints a human-readable report.
func FormatResult(r *Result) string {
	if r == nil {
		return "no field mismatch at a node whose inputs all match (paths may differ only by hash)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "deepest mismatch (depth %d) logical=%q\n", r.Depth, r.LogicalName)
	fmt.Fprintf(&b, "  nix key:    %s\n", r.NixKey)
	fmt.Fprintf(&b, "  goonix key: %s\n", r.GoonixKey)
	for _, d := range r.Diffs {
		fmt.Fprintf(&b, "  %s:\n    nix:    %s\n    goonix: %s\n", d.Path, d.Nix, d.Goonix)
	}
	return b.String()
}
