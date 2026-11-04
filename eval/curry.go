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
	fv, err := ev.forceValue(f)
	if err != nil {
		return nil, err
	}
	switch fv.Kind {
	case VKLambda:
		return ev.applyLambda(fv.Lambda, []*Value{arg}, pos)
	case VKBuiltin:
		return ev.applyBuiltinStep(fv.Builtin, nil, arg, pos)
	case VKPartialBuiltin:
		return ev.applyBuiltinStep(fv.Partial.B, fv.Partial.Args, arg, pos)
	default:
		return nil, evalErr(pos, "attempt to call non-function")
	}
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
