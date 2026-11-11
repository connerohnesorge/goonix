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
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	setsList, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	var sets []*Value
	for _, el := range setsList {
		v, err := ev.forceAttrs(el, pos)
		if err != nil {
			return nil, err
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
		sym := sym
		inSets := sets
		t := &Thunk{
			eval: ev,
			pos:  pos,
			lazy: func(ev *Evaluator) (*Value, error) {
				name := ev.Intern.Lookup(sym)
				var vals []*Value
				for _, s := range inSets {
					if b, ok := s.Attrs.Binds[sym]; ok && b != nil {
						// The list f receives holds the values unevaluated.
						if b.Thunk != nil {
							vals = append(vals, Deferred(b.Thunk))
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
				return ev.forceValue(r)
			},
		}
		out.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
	}
	return out, nil
}
