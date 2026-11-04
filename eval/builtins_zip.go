// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"sort"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
)

func builtinZipAttrsWith(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	setsList, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	var sets []*Value
	for _, el := range setsList {
		v, err := ev.forceValue(el)
		if err != nil {
			return nil, err
		}
		if v.Kind != VKAttrs {
			return nil, evalErr(pos, "zipAttrsWith: expected list of attribute sets")
		}
		sets = append(sets, v)
	}

	names := map[data.Symbol]struct{}{}
	for _, s := range sets {
		for sym := range s.Attrs.Binds {
			names[sym] = struct{}{}
		}
	}
	syms := make([]data.Symbol, 0, len(names))
	for sym := range names {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		return ev.Intern.Lookup(syms[i]) < ev.Intern.Lookup(syms[j])
	})

	out := Attrs(false)
	for _, sym := range syms {
		name := ev.Intern.Lookup(sym)
		var vals []*Value
		for _, s := range sets {
			if b, ok := s.Attrs.Binds[sym]; ok && b != nil {
				if b.Thunk != nil {
					v, err := ev.forceThunk(b.Thunk)
					if err != nil {
						return nil, err
					}
					vals = append(vals, v)
				} else if b.Value != nil {
					vals = append(vals, b.Value)
				}
			}
		}
		nameVal := Str(name, pos)
		listVal := List(vals...)
		r, err := ev.applyCall2(f, nameVal, listVal, pos)
		if err != nil {
			return nil, err
		}
		rv, err := ev.forceValue(r)
		if err != nil {
			return nil, err
		}
		out.Attrs.Binds[sym] = &AttrBinding{Value: rv}
	}
	return out, nil
}
