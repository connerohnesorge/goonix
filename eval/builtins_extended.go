// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"math"

	"github.com/connerohnesorge/goonix/parser"
)

func (ev *Evaluator) registerExtendedBuiltins() {
	add := func(name string, fn func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error), nullary bool) {
		if ev.Builtins[name] != nil {
			return
		}
		ev.Builtins[name] = &Builtin{Name: name, Nullary: nullary, Fn: fn}
	}
	add("isNull", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		if err := arity(pos, args, 1); err != nil {
			return nil, err
		}
		v, err := ev.forceValue(args[0])
		if err != nil {
			return nil, err
		}
		return Bool(v.Kind == VKNull), nil
	}, false)
	add("langVersion", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		return Int(5), nil
	}, true)
	add("warn", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		if err := arity(pos, args, 2); err != nil {
			return nil, err
		}
		return args[1], nil
	}, false)
	add("true", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		return Bool(true), nil
	}, true)
	add("false", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		return Bool(false), nil
	}, true)
	add("null", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		return Null(), nil
	}, true)
	add("builtins", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		return ev.BuiltinAttrSet(), nil
	}, true)
	add("hasContext", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		if err := arity(pos, args, 1); err != nil {
			return nil, err
		}
		return Bool(false), nil
	}, false)
	add("ceil", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		if err := arity(pos, args, 1); err != nil {
			return nil, err
		}
		v, err := ev.forceValue(args[0])
		if err != nil {
			return nil, err
		}
		switch v.Kind {
		case VKInt:
			return v, nil
		case VKFloat:
			return Int(int64(math.Ceil(v.Float))), nil
		default:
			return nil, evalErr(pos, "ceil expects a number")
		}
	}, false)
	add("floor", func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
		if err := arity(pos, args, 1); err != nil {
			return nil, err
		}
		v, err := ev.forceValue(args[0])
		if err != nil {
			return nil, err
		}
		switch v.Kind {
		case VKInt:
			return v, nil
		case VKFloat:
			return Int(int64(math.Floor(v.Float))), nil
		default:
			return nil, evalErr(pos, "floor expects a number")
		}
	}, false)
	for _, name := range []string{
		"groupBy", "partition", "convertHash", "hashFile", "fromTOML", "toXML",
		"readFileType", "scopedImport", "fetchTarball", "fetchGit", "fetchMercurial",
		"fetchTree", "filterAttrs", "toPath", "traceVerbose", "addDrvOutputDependencies",
		"break", "currentTime", "getFlake", "flakeRefToString", "parseFlakeRef",
		"unsafeDiscardOutputDependency",
	} {
		n := name
		add(n, func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
			return nil, evalErr(pos, "builtins.%s is not implemented", n)
		}, false)
	}
}
