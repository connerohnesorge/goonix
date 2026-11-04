// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"errors"
	"regexp"
	"strconv"

	"github.com/connerohnesorge/goonix/parser"
)

// PartialBuiltin is a curried builtin waiting for more arguments.
type PartialBuiltin struct {
	B    *Builtin
	Args []*Value
}

var arityErrRe = regexp.MustCompile(`expected (\d+) arguments, got (\d+)`)

func parseArityError(err error) (expected, got int, ok bool) {
	var ee *EvalError
	if !errors.As(err, &ee) {
		return 0, 0, false
	}
	m := arityErrRe.FindStringSubmatch(ee.Msg)
	if m == nil {
		return 0, 0, false
	}
	exp, _ := strconv.Atoi(m[1])
	g, _ := strconv.Atoi(m[2])
	return exp, g, true
}

// applyCallable applies one argument to a function-like value.
func (ev *Evaluator) applyCallable(f *Value, arg *Value, pos parser.Pos) (*Value, error) {
	fv, err := ev.mustWHNF(f, pos)
	if err != nil {
		return nil, err
	}
	if !ev.isCallableValue(fv, pos) {
		ev.logTypeMismatchWant(pos, fv, "function")
		return nil, evalErr(pos, "attempt to call non-function")
	}
	switch fv.Kind {
	case VKLambda:
		return ev.applyLambda(fv.Lambda, []*Value{arg}, pos)
	case VKBuiltin:
		return ev.applyBuiltinStep(fv.Builtin, nil, arg, pos)
	case VKPartialBuiltin:
		return ev.applyBuiltinStep(fv.Partial.B, fv.Partial.Args, arg, pos)
	case VKAttrs:
		if fn, ok, err := ev.functorApply(fv, arg, pos); ok || err != nil {
			return fn, err
		}
		return nil, evalErr(pos, "attempt to call non-function")
	default:
		return nil, evalErr(pos, "attempt to call non-function")
	}
}

func (ev *Evaluator) isCallableValue(fv *Value, pos parser.Pos) bool {
	return ev.isFunctionValue(fv, pos)
}

func (ev *Evaluator) hasFunctor(v *Value) bool {
	if v == nil || v.Kind != VKAttrs || v.Attrs == nil {
		return false
	}
	sym := ev.Intern.Intern("__functor")
	_, ok := v.Attrs.Binds[sym]
	return ok
}

// functorApply implements Nix's __functor calling convention: (f arg) == (f.__functor f) arg.
func (ev *Evaluator) functorApply(self *Value, arg *Value, pos parser.Pos) (*Value, bool, error) {
	if !ev.hasFunctor(self) {
		return nil, false, nil
	}
	sym := ev.Intern.Intern("__functor")
	b := self.Attrs.Binds[sym]
	var functor *Value
	if b.Thunk != nil {
		v, err := ev.forceThunk(b.Thunk)
		if err != nil {
			return nil, true, err
		}
		functor = v
	} else {
		functor = b.Value
	}
	fn, err := ev.applyCallable(functor, self, pos)
	if err != nil {
		return nil, true, err
	}
	out, err := ev.applyCallable(fn, arg, pos)
	return out, true, err
}

func (ev *Evaluator) applyBuiltinStep(b *Builtin, bound []*Value, arg *Value, pos parser.Pos) (*Value, error) {
	args := append(append([]*Value{}, bound...), arg)
	v, err := b.Fn(ev, pos, args)
	if err == nil {
		return v, nil
	}
	exp, got, ok := parseArityError(err)
	if ok && got < exp {
		return &Value{
			Kind:    VKPartialBuiltin,
			Partial: &PartialBuiltin{B: b, Args: args},
		}, nil
	}
	return nil, err
}

func (ev *Evaluator) applyCallableN(f *Value, args []*Value, pos parser.Pos) (*Value, error) {
	cur := f
	var err error
	for _, a := range args {
		cur, err = ev.applyCallable(cur, a, pos)
		if err != nil {
			return nil, err
		}
	}
	return cur, nil
}
