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
	PairID      string
	LogicalName string
	NixKey      string
	GoonixKey   string
	NixDrvPath  string
	GoonixPath  string
	NixATerm    []byte
	GoonixATerm []byte
	EnvDiffs    []FieldDiff
}

// CompareBytesBottomUp finds the first ATerm mismatch in topological (leaves-first) order.
func CompareBytesBottomUp(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string, src ATermBytes) (*ByteCulprit, error) {
	pairsAll := pairClosureByLogical(nixDoc, goDoc)
	return compareBytesBottomUpFromRoot(nixDoc, goDoc, nixRootKey, pairsAll, logicalPairID(nixDoc, nixRootKey), src, func(key string) string {
		return drvshow.LogicalName(key)
	})
}

// CompareBytesBottomUpNameStdenv pairs nodes by drvshow.NameStdenvPairID (name + env.stdenv stage).
func CompareBytesBottomUpNameStdenv(nixDoc, goDoc drvshow.Document, nixRootKey, goRootKey string, src ATermBytes) (*ByteCulprit, error) {
	pairsAll := pairClosureByNameStdenvFromRoot(nixDoc, goDoc, nixRootKey, goRootKey)
	nixKeyToID := make(map[string]string, len(pairsAll))
	for id, p := range pairsAll {
		nixKeyToID[p.nixKey] = id
	}
	idOfKey := func(key string) string {
		if id, ok := nixKeyToID[key]; ok {
			return id
		}
		return nameStdenvPairID(nixDoc, key)
	}
	rootID := idOfKey(nixRootKey)
	return compareBytesBottomUpFromRoot(nixDoc, goDoc, nixRootKey, pairsAll, rootID, src, idOfKey)
}

func logicalPairID(doc drvshow.Document, key string) string {
	return drvshow.LogicalName(key)
}

func nameStdenvPairID(doc drvshow.Document, key string) string {
	e, ok := doc.Derivations[key]
	if !ok {
		return ""
	}
	return drvshow.NameStdenvPairID(e)
}

func compareBytesBottomUpFromRoot(nixDoc, goDoc drvshow.Document, nixRootKey string, pairsAll map[string]keyPair, rootID string, src ATermBytes, idOfKey func(string) string) (*ByteCulprit, error) {
	if pairsAll == nil {
		return nil, fmt.Errorf("no paired derivations")
	}
	closure := closurePairIDs(nixDoc, nixRootKey, idOfKey)
	pairs := make(map[string]keyPair, len(closure))
	for id := range closure {
		p, ok := pairsAll[id]
		if !ok {
			continue
		}
		pairs[id] = p
	}
	if _, ok := pairs[rootID]; !ok {
		return nil, fmt.Errorf("root pair id %q not paired (nix=%s)", rootID, nixRootKey)
	}
	return compareBytesBottomUpPairs(nixDoc, goDoc, pairs, idOfKey, src)
}

func closurePairIDs(doc drvshow.Document, rootKey string, idOfKey func(string) string) map[string]struct{} {
	out := make(map[string]struct{})
	var walk func(string)
	walk = func(key string) {
		if key == "" {
			return
		}
		id := idOfKey(key)
		if id == "" {
			return
		}
		if _, ok := out[id]; ok {
			return
		}
		out[id] = struct{}{}
		e, ok := doc.Derivations[key]
		if !ok {
			return
		}
		for dk := range e.Inputs.Drvs {
			walk(dk)
		}
	}
	walk(rootKey)
	return out
}

func compareBytesBottomUpPairs(nixDoc, goDoc drvshow.Document, pairs map[string]keyPair, idOfKey func(string) string, src ATermBytes) (*ByteCulprit, error) {
	deps := make(map[string][]string, len(pairs))
	for id, p := range pairs {
		n := nixDoc.Derivations[p.nixKey]
		seen := map[string]struct{}{}
		for dk := range n.Inputs.Drvs {
			depID := idOfKey(dk)
			if depID == "" || depID == id {
				continue
			}
			if _, ok := pairs[depID]; !ok {
				continue
			}
			if _, ok := seen[depID]; ok {
				continue
			}
			seen[depID] = struct{}{}
			deps[id] = append(deps[id], depID)
		}
		sort.Strings(deps[id])
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

	for _, id := range order {
		p := pairs[id]
		nb := src.NixATerm(p.nixKey)
		gb := src.GoATerm(p.goKey)
		if len(nb) == 0 || len(gb) == 0 {
			continue
		}
		if string(nb) == string(gb) {
			continue
		}
		inputsOK := true
		for _, dep := range deps[id] {
			dp := pairs[dep]
			if !bytesEqual(dp.nixKey, dp.goKey) {
				inputsOK = false
				break
			}
		}
		if !inputsOK {
			continue
		}
		nEntry := nixDoc.Derivations[p.nixKey]
		gEntry := goDoc.Derivations[p.goKey]
		envDiffs := fieldDiffs(nEntry, gEntry)
		return &ByteCulprit{
			PairID:      id,
			LogicalName: drvshow.LogicalName(p.nixKey),
			NixKey:      p.nixKey,
			GoonixKey:   p.goKey,
			NixDrvPath:  src.NixDrvPath(p.nixKey),
			GoonixPath:  src.GoDrvPath(p.goKey),
			NixATerm:    nb,
			GoonixATerm: gb,
			EnvDiffs:    envDiffs,
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

func pairClosureByNameStdenvFromRoot(nixDoc, goDoc drvshow.Document, nixRoot, goRoot string) map[string]keyPair {
	nixLists := groupKeysByPairID(nixDoc, nixRoot)
	goLists := groupKeysByPairID(goDoc, goRoot)
	out := make(map[string]keyPair)
	for id, nKeys := range nixLists {
		gKeys := goLists[id]
		if len(gKeys) == 0 {
			continue
		}
		n := len(nKeys)
		if len(gKeys) < n {
			n = len(gKeys)
		}
		if n == 1 {
			out[id] = keyPair{nixKey: nKeys[0], goKey: gKeys[0]}
			continue
		}
		for i := 0; i < n; i++ {
			suffix := string(rune('a' + i))
			if i >= 26 {
				suffix = fmt.Sprintf("%d", i)
			}
			out[id+"\x1f"+suffix] = keyPair{nixKey: nKeys[i], goKey: gKeys[i]}
		}
	}
	return out
}

func groupKeysByPairID(doc drvshow.Document, root string) map[string][]string {
	lists := make(map[string][]string)
	for _, k := range drvshow.ClosureBFSOrder(doc, root) {
		e, ok := doc.Derivations[k]
		if !ok {
			continue
		}
		id := drvshow.NameStdenvPairID(e)
		lists[id] = append(lists[id], k)
	}
	return lists
}

func pairClosureByNameStdenv(nixDoc, goDoc drvshow.Document) map[string]keyPair {
	// Legacy pairing without roots: first key per pair ID in arbitrary map order.
	nixBy := make(map[string]string)
	for k, e := range nixDoc.Derivations {
		id := drvshow.NameStdenvPairID(e)
		if _, ok := nixBy[id]; !ok {
			nixBy[id] = k
		}
	}
	out := make(map[string]keyPair)
	for k, e := range goDoc.Derivations {
		id := drvshow.NameStdenvPairID(e)
		nk, ok := nixBy[id]
		if !ok {
			continue
		}
		out[id] = keyPair{nixKey: nk, goKey: k}
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
	fmt.Fprintf(&b, "first byte mismatch (inputs match) pair=%q logical=%q\n", c.PairID, c.LogicalName)
	fmt.Fprintf(&b, "  nix key:    %s\n", c.NixKey)
	fmt.Fprintf(&b, "  goonix key: %s\n", c.GoonixKey)
	fmt.Fprintf(&b, "  nix drvPath:    %s\n", c.NixDrvPath)
	fmt.Fprintf(&b, "  goonix drvPath: %s\n", c.GoonixPath)
	for _, d := range c.EnvDiffs {
		if strings.HasPrefix(d.Path, "env.") {
			fmt.Fprintf(&b, "  %s nix=%q go=%q\n", d.Path, truncField(d.Nix, 200), truncField(d.Goonix, 200))
		}
	}
	b.WriteString(formatByteDiff(c.NixATerm, c.GoonixATerm, 120))
	return b.String()
}

func truncField(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
