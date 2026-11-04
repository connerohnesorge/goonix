// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"github.com/connerohnesorge/goonix/parser"
)

// insertBinding places bind into av, creating nested attrsets for multi-segment paths.
func (ev *Evaluator) insertBinding(av *Value, bindEnv *Env, bind parser.Binding, recursive bool) {
	if len(bind.Path) == 0 {
		return
	}
	if bind.Path[0].Expr != nil {
		return
	}
	t := ev.newThunk(bind.Value, bindEnv, bind.Pos)
	if len(bind.Path) == 1 {
		sym := ev.Intern.Intern(bind.Path[0].Name)
		t.selfHideSym = sym
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		if recursive {
			bindEnv.bindThunk(sym, t)
		}
		return
	}
	nested := ev.ensureNestedAttr(av, bind.Path[0].Name, recursive, bindEnv)
	ev.insertBindingInto(nested, bindEnv, bind.Path[1:], t, recursive)
}

func (ev *Evaluator) ensureNestedAttr(av *Value, name string, recursive bool, bindEnv *Env) *Value {
	sym := ev.Intern.Intern(name)
	if b, ok := av.Attrs.Binds[sym]; ok && b.Value != nil && b.Value.Kind == VKAttrs {
		return b.Value
	}
	nested := Attrs(recursive)
	av.Attrs.Binds[sym] = &AttrBinding{Value: nested}
	if recursive {
		bindEnv.bind(sym, nested)
	}
	return nested
}

func (ev *Evaluator) insertBindingInto(av *Value, bindEnv *Env, path []parser.AttrName, t *Thunk, recursive bool) {
	if len(path) == 0 {
		return
	}
	if path[0].Expr != nil {
		return
	}
	if len(path) == 1 {
		sym := ev.Intern.Intern(path[0].Name)
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		return
	}
	nested := ev.ensureNestedAttr(av, path[0].Name, recursive, bindEnv)
	ev.insertBindingInto(nested, bindEnv, path[1:], t, recursive)
}

func (ev *Evaluator) syncRecTopLevel(av *Value, bindEnv *Env) {
	for sym, bind := range av.Attrs.Binds {
		if bind.Value != nil && bind.Value.Kind == VKAttrs {
			name := ev.Intern.Lookup(sym)
			if name != "" {
				bindEnv.bind(sym, bind.Value)
			}
		}
	}
}
