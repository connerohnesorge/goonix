// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
)

func (ev *Evaluator) resolveAttrSym(an parser.AttrName, env *Env, pos parser.Pos) (data.Symbol, error) {
	if an.Expr == nil {
		return ev.Intern.Intern(an.Name), nil
	}
	v, err := ev.evalExpr(an.Expr, env)
	if err != nil {
		return 0, err
	}
	s, err := ev.forceString(v)
	if err != nil {
		return 0, evalErr(pos, "dynamic attribute name must be a string")
	}
	return ev.Intern.Intern(s), nil
}

// insertBinding places bind into av, creating nested attrsets for multi-segment paths.
func (ev *Evaluator) insertBinding(av *Value, bindEnv *Env, bind parser.Binding, recursive bool) error {
	if len(bind.Path) == 0 {
		return nil
	}
	sym, err := ev.resolveAttrSym(bind.Path[0], bindEnv, bind.Pos)
	if err != nil {
		return err
	}
	t := ev.newThunk(bind.Value, bindEnv, bind.Pos)
	if len(bind.Path) == 1 {
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		if recursive {
			bindEnv.bindThunk(sym, t)
		}
		return nil
	}
	name := ev.Intern.Lookup(sym)
	nested := ev.ensureNestedAttr(av, name, recursive, bindEnv)
	return ev.insertBindingInto(nested, bindEnv, bind.Path[1:], t, recursive)
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

func (ev *Evaluator) insertBindingInto(av *Value, bindEnv *Env, path []parser.AttrName, t *Thunk, recursive bool) error {
	if len(path) == 0 {
		return nil
	}
	sym, err := ev.resolveAttrSym(path[0], bindEnv, t.pos)
	if err != nil {
		return err
	}
	if len(path) == 1 {
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		return nil
	}
	name := ev.Intern.Lookup(sym)
	nested := ev.ensureNestedAttr(av, name, recursive, bindEnv)
	return ev.insertBindingInto(nested, bindEnv, path[1:], t, recursive)
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
