// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"github.com/connerohnesorge/goonix/parser"
)

// forceWHNF peels thunks to WHNF (same as forceValue).
func (ev *Evaluator) forceWHNF(v *Value, pos parser.Pos) (*Value, error) {
	_ = pos
	return ev.forceValue(v)
}

// mustWHNF requires a WHNF head (never a bare thunk).
func (ev *Evaluator) mustWHNF(v *Value, pos parser.Pos) (*Value, error) {
	fv, err := ev.forceValue(v)
	if err != nil {
		return nil, err
	}
	if fv != nil && fv.Kind == VKDeferred {
		ev.logTypeMismatchWant(pos, fv, "WHNF head (not thunk)")
		return nil, evalErr(pos, "infinite recursion encountered")
	}
	return fv, nil
}

// forceAttrs is WHNF coercion to an attribute set (Nix forceAttrs).
func (ev *Evaluator) forceAttrs(v *Value, pos parser.Pos) (*Value, error) {
	fv, err := ev.mustWHNF(v, pos)
	if err != nil {
		return nil, err
	}
	if fv == nil || fv.Kind != VKAttrs {
		ev.logTypeMismatchWant(pos, fv, "attribute set")
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
