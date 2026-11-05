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
	add("langVersion", builtinLangVersion6, true)
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
	add("hasContext", builtinHasContextReal, false)
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
	add("groupBy", builtinGroupBy, false)
	add("partition", builtinPartition, false)
	add("convertHash", builtinConvertHash, false)
	add("hashFile", builtinHashFile, false)
	add("fromTOML", builtinFromTOML, false)
	add("toXML", builtinToXML, false)
	add("readFileType", builtinReadFileType, false)
	add("scopedImport", builtinScopedImport, false)
	add("fetchTarball", builtinFetchTarball, false)
	add("fetchGit", builtinFetchGit, false)
	add("fetchMercurial", builtinFetchMercurial, false)
	add("toPath", builtinToPath, false)
	add("traceVerbose", builtinTraceVerbose, false)
	add("addDrvOutputDependencies", builtinAddDrvOutputDependencies, false)
	add("break", builtinBreak, false)
	add("currentTime", builtinCurrentTime, true)
	add("unsafeDiscardOutputDependency", builtinUnsafeDiscardOutputDependency, false)
	ev.syncExperimentalBuiltins()
}

// syncExperimentalBuiltins adds or removes primops the way Nix 2.24.9 does:
// they are absent from builtins unless the experimental feature is enabled.
// flakes also enables fetch-tree.
func (ev *Evaluator) syncExperimentalBuiltins() {
	for _, name := range []string{"fetchTree", "getFlake", "parseFlakeRef", "flakeRefToString"} {
		delete(ev.Builtins, name)
	}
	if ev.xp != nil && (ev.xp["fetch-tree"] || ev.xp["flakes"]) {
		ev.Builtins["fetchTree"] = &Builtin{Name: "fetchTree", Fn: builtinFetchTree}
	}
	if ev.xp != nil && ev.xp["flakes"] {
		ev.Builtins["getFlake"] = &Builtin{Name: "getFlake", Fn: builtinGetFlake}
		ev.Builtins["parseFlakeRef"] = &Builtin{Name: "parseFlakeRef", Fn: builtinParseFlakeRef}
		ev.Builtins["flakeRefToString"] = &Builtin{Name: "flakeRefToString", Fn: builtinFlakeRefToString}
	}
}
