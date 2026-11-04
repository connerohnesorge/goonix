// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"github.com/connerohnesorge/goonix/parser"
)

// forceWHNF forces v to weak head normal form for select / hasAttr (Nix: peel
// thunks only; never force inside aggregates). Stops on locked fixpoint thunks.
func (ev *Evaluator) forceWHNF(v *Value, pos parser.Pos) (*Value, error) {
	_ = pos
	const maxSteps = 64
	seen := make(map[*Thunk]struct{})
	for step := 0; step < maxSteps && v != nil && v.Kind == VKDeferred && v.Deferred != nil; step++ {
		t := v.Deferred
		if t.locked {
			return v, nil
		}
		if _, ok := seen[t]; ok {
			return v, nil
		}
		seen[t] = struct{}{}
		var err error
		v, err = ev.forceValue(v)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

// forceAttrs is WHNF coercion to an attribute set (Nix forceAttrs).
func (ev *Evaluator) forceAttrs(v *Value, pos parser.Pos) (*Value, error) {
	fv, err := ev.forceValue(v)
	if err != nil {
		return nil, err
	}
	if fv == nil || fv.Kind != VKAttrs {
		return nil, evalErr(pos, "// expects attribute sets")
	}
	return fv, nil
}

// evalAttrs evaluates e and demands the result is an attribute set (Nix evalAttrs).
func (ev *Evaluator) evalAttrs(e parser.Expr, env *Env, pos parser.Pos) (*Value, error) {
	v, err := ev.evalExpr(e, env)
	if err != nil {
		return nil, err
	}
	return ev.forceAttrs(v, pos)
}

// mergeAttrSets merges lhs // rhs binding maps without forcing attribute values.
func (ev *Evaluator) mergeAttrSets(l, r *Value, pos parser.Pos) (*Value, error) {
	fl, err := ev.forceAttrs(l, pos)
	if err != nil {
		return nil, err
	}
	fr, err := ev.forceAttrs(r, pos)
	if err != nil {
		return nil, err
	}
	return mergeAttrBindMaps(fl, fr), nil
}
