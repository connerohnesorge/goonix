// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvdiff

import (
	"fmt"
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/store/drvshow"
)

// ATermBytes loads raw .drv file bodies for paired keys.
type ATermBytes interface {
	NixATerm(nixDrvKey string) []byte
	GoATerm(goDrvKey string) []byte
	NixDrvPath(nixDrvKey string) string
	GoDrvPath(goDrvKey string) string
}

// ByteCulprit is the first paired node in leaf-first order whose ATerm bytes differ
// while every paired input derivation already matches byte-for-byte.
type ByteCulprit struct {
	LogicalName string
	NixKey      string
	GoonixKey   string
	NixDrvPath  string
	GoonixPath  string
	NixATerm    []byte
	GoonixATerm []byte
}

// CompareBytesBottomUp finds the first ATerm mismatch in topological (leaves-first) order.
func CompareBytesBottomUp(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string, src ATermBytes) (*ByteCulprit, error) {
	pairs := pairClosureByLogical(nixDoc, goDoc)
	if pairs == nil {
		return nil, fmt.Errorf("no paired derivations")
	}
	rootLogical := drvshow.LogicalName(nixRootKey)
	if _, ok := pairs[rootLogical]; !ok {
		return nil, fmt.Errorf("root logical %q not paired", rootLogical)
	}

	deps := make(map[string][]string, len(pairs))
	for logical, p := range pairs {
		n := nixDoc.Derivations[p.nixKey]
		seen := map[string]struct{}{}
		for k := range n.Inputs.Drvs {
			dep := drvshow.LogicalName(k)
			if dep == logical {
				continue
			}
			if _, ok := pairs[dep]; !ok {
				continue
			}
			if _, ok := seen[dep]; ok {
				continue
			}
			seen[dep] = struct{}{}
			deps[logical] = append(deps[logical], dep)
		}
		sort.Strings(deps[logical])
	}

	order, err := topoLeavesFirstByDepth(deps, pairs)
	if err != nil {
		return nil, err
	}

	bytesEqual := func(nixKey, goKey string) bool {
		nb := src.NixATerm(nixKey)
		gb := src.GoATerm(goKey)
		return len(nb) > 0 && len(gb) > 0 && string(nb) == string(gb)
	}

	for _, logical := range order {
		p := pairs[logical]
		nb := src.NixATerm(p.nixKey)
		gb := src.GoATerm(p.goKey)
		if len(nb) == 0 || len(gb) == 0 {
			continue
		}
		if string(nb) == string(gb) {
			continue
		}
		inputsOK := true
		for _, dep := range deps[logical] {
			dp := pairs[dep]
			if !bytesEqual(dp.nixKey, dp.goKey) {
				inputsOK = false
				break
			}
		}
		if !inputsOK {
			continue
		}
		return &ByteCulprit{
			LogicalName: logical,
			NixKey:      p.nixKey,
			GoonixKey:   p.goKey,
			NixDrvPath:  src.NixDrvPath(p.nixKey),
			GoonixPath:  src.GoDrvPath(p.goKey),
			NixATerm:    nb,
			GoonixATerm: gb,
		}, nil
	}
	return nil, nil
}

type keyPair struct {
	nixKey string
	goKey  string
}

func pairClosureByLogical(nixDoc, goDoc drvshow.Document) map[string]keyPair {
	nixBy := make(map[string]string)
	for k := range nixDoc.Derivations {
		nixBy[drvshow.LogicalName(k)] = k
	}
	out := make(map[string]keyPair)
	for k := range goDoc.Derivations {
		logical := drvshow.LogicalName(k)
		nk, ok := nixBy[logical]
		if !ok {
			continue
		}
		out[logical] = keyPair{nixKey: nk, goKey: k}
	}
	return out
}

// topoLeavesFirstByDepth sorts by max input-chain depth (leaves first), stable by name.
func topoLeavesFirstByDepth(deps map[string][]string, pairs map[string]keyPair) ([]string, error) {
	memo := make(map[string]int)
	visiting := make(map[string]bool)
	var depth func(string) int
	depth = func(logical string) int {
		if d, ok := memo[logical]; ok {
			return d
		}
		if visiting[logical] {
			return 0
		}
		visiting[logical] = true
		defer delete(visiting, logical)
		ds := deps[logical]
		if len(ds) == 0 {
			memo[logical] = 0
			return 0
		}
		max := 0
		for _, d := range ds {
			max = maxInt(max, depth(d)+1)
		}
		memo[logical] = max
		return max
	}
	order := make([]string, 0, len(pairs))
	for logical := range pairs {
		order = append(order, logical)
	}
	sort.Slice(order, func(i, j int) bool {
		di, dj := depth(order[i]), depth(order[j])
		if di != dj {
			return di < dj
		}
		return order[i] < order[j]
	})
	return order, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// topoLeavesFirst returns nodes in an order where every dependency appears before its dependents.
func topoLeavesFirst(deps map[string][]string, pairs map[string]keyPair) ([]string, error) {
	inDegree := make(map[string]int, len(pairs))
	for logical := range pairs {
		inDegree[logical] = 0
	}
	dependents := make(map[string][]string)
	for logical, ds := range deps {
		for _, d := range ds {
			dependents[d] = append(dependents[d], logical)
			inDegree[logical]++
		}
	}
	var queue []string
	for logical, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, logical)
		}
	}
	sort.Strings(queue)
	var order []string
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		order = append(order, n)
		for _, dep := range dependents[n] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
				sort.Strings(queue)
			}
		}
	}
	if len(order) != len(pairs) {
		return nil, fmt.Errorf("cycle in derivation graph (ordered %d of %d)", len(order), len(pairs))
	}
	return order, nil
}

// FormatByteCulprit reports the culprit and a contextual byte diff of ATerm bodies.
func FormatByteCulprit(c *ByteCulprit) string {
	if c == nil {
		return "all paired .drv ATerm bodies match (leaf-first scan)\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "first byte mismatch (inputs match) logical=%q\n", c.LogicalName)
	fmt.Fprintf(&b, "  nix key:    %s\n", c.NixKey)
	fmt.Fprintf(&b, "  goonix key: %s\n", c.GoonixKey)
	fmt.Fprintf(&b, "  nix drvPath:    %s\n", c.NixDrvPath)
	fmt.Fprintf(&b, "  goonix drvPath: %s\n", c.GoonixPath)
	b.WriteString(formatByteDiff(c.NixATerm, c.GoonixATerm, 120))
	return b.String()
}

func formatByteDiff(nix, goonix []byte, context int) string {
	ns, gs := string(nix), string(goonix)
	if ns == gs {
		return "  (identical)\n"
	}
	i := 0
	for i < len(ns) && i < len(gs) && ns[i] == gs[i] {
		i++
	}
	start := i - context
	if start < 0 {
		start = 0
	}
	endN := i + context
	if endN > len(ns) {
		endN = len(ns)
	}
	endG := i + context
	if endG > len(gs) {
		endG = len(gs)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "  first differing byte at offset %d (nix len=%d go len=%d)\n", i, len(ns), len(gs))
	fmt.Fprintf(&b, "  --- nix[%d:%d] ---\n%s\n", start, endN, ns[start:endN])
	fmt.Fprintf(&b, "  --- go[%d:%d] ---\n%s\n", start, endG, gs[start:endG])
	if len(ns) != len(gs) {
		fmt.Fprintf(&b, "  suffix: nix tail from %d (%d bytes), go tail from %d (%d bytes)\n", i, len(ns)-i, i, len(gs)-i)
	}
	return b.String()
}

// StoreATermSource reads nix from /nix/store and go from a callback.
type StoreATermSource struct {
	Go func(goDrvKey string) []byte
}

func (s StoreATermSource) NixATerm(nixDrvKey string) []byte {
	return ReadNixStoreATerm(nixDrvKey)
}

func (s StoreATermSource) GoATerm(goDrvKey string) []byte {
	if s.Go == nil {
		return nil
	}
	return s.Go(goDrvKey)
}

func (StoreATermSource) NixDrvPath(nixDrvKey string) string {
	return "/nix/store/" + nixDrvKey
}

func (StoreATermSource) GoDrvPath(goDrvKey string) string {
	return "/nix/store/" + goDrvKey
}
