// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"reflect"
	"sort"
)

// WithVerifyStrictMemo enables test-mode checks on derivationStrict memo hits:
// recompute without using the cache and assert bit-identical results.
func (ev *Evaluator) WithVerifyStrictMemo(v bool) *Evaluator {
	c := *ev
	c.verifyStrictMemo = v
	return &c
}

func (ev *Evaluator) verifyStrictMemoMatch(attrs *Value, cached *DerivationValue) error {
	if attrs == nil || attrs.Kind != VKAttrs || attrs.Attrs == nil || ev.drvStrictByAttrs == nil {
		return nil
	}
	delete(ev.drvStrictByAttrs, attrs.Attrs)
	fresh, err := ev.computeAttrsToDerivation(attrs)
	ev.drvStrictByAttrs[attrs.Attrs] = cached
	if err != nil {
		return fmt.Errorf("derivationStrict memo recompute: %w", err)
	}
	if err := compareDerivationValues(cached, fresh); err != nil {
		return fmt.Errorf("derivationStrict memo nondeterministic: %w", err)
	}
	return nil
}

func compareDerivationValues(a, b *DerivationValue) error {
	if a == nil || b == nil {
		return fmt.Errorf("nil derivation")
	}
	if a.Name != b.Name || a.System != b.System || a.Builder != b.Builder {
		return fmt.Errorf("scalar mismatch name/system/builder")
	}
	if a.DrvPath != b.DrvPath {
		return fmt.Errorf("drvPath %q vs %q", a.DrvPath, b.DrvPath)
	}
	if !reflect.DeepEqual(a.Args, b.Args) {
		return fmt.Errorf("args differ")
	}
	if !reflect.DeepEqual(a.Outputs, b.Outputs) {
		return fmt.Errorf("outputs differ")
	}
	if !reflect.DeepEqual(a.OutPaths, b.OutPaths) {
		return fmt.Errorf("outPaths differ")
	}
	if !reflect.DeepEqual(a.InputDerivations, b.InputDerivations) {
		return fmt.Errorf("inputDrvs differ")
	}
	if !reflect.DeepEqual(a.InputSources, b.InputSources) {
		return fmt.Errorf("inputSrcs differ")
	}
	if len(a.Env) != len(b.Env) {
		return fmt.Errorf("env len %d vs %d", len(a.Env), len(b.Env))
	}
	keys := make([]string, 0, len(a.Env))
	for k := range a.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if a.Env[k] != b.Env[k] {
			return fmt.Errorf("env[%q]", k)
		}
	}
	return nil
}
