// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
)

// Env is a lexical environment frame.
type Env struct {
	Parent *Env
	// Static bindings from let/rec/lambda (eager name -> value or thunk).
	Vars map[data.Symbol]*AttrBinding
	// Dynamic layer from `with` (attrs value, looked up after Vars).
	With *Value
}

// NewEnv returns an empty environment.
func NewEnv() *Env {
	return &Env{Vars: make(map[data.Symbol]*AttrBinding)}
}

func (e *Env) extend() *Env {
	return &Env{
		Parent: e,
		Vars:   make(map[data.Symbol]*AttrBinding),
	}
}

func (e *Env) bind(sym data.Symbol, v *Value) {
	e.Vars[sym] = &AttrBinding{Value: v}
}

func (e *Env) bindThunk(sym data.Symbol, t *Thunk) {
	e.Vars[sym] = &AttrBinding{Thunk: t}
}

func (e *Env) lookup(sym data.Symbol) (*AttrBinding, bool) {
	for cur := e; cur != nil; cur = cur.Parent {
		if b, ok := cur.Vars[sym]; ok {
			return b, true
		}
	}
	return nil, false
}

func (ev *Evaluator) lookupEnv(env *Env, sym data.Symbol) (*AttrBinding, bool) {
	if env == nil {
		return nil, false
	}
	if b, ok := env.Vars[sym]; ok {
		if ev.forceHideSym != 0 && sym == ev.forceHideSym && ev.inMergeLeft &&
			b.Thunk != nil && ev.currentForce != nil && b.Thunk == ev.currentForce {
			return &AttrBinding{Value: Deferred(ev.currentForce)}, true
		}
		return b, true
	}
	if b, ok := ev.lookupEnv(env.Parent, sym); ok {
		return b, true
	}
	if env.With != nil {
		var w *Value
		var err error
		if env.With.Kind == VKDeferred && env.With.Deferred != nil {
			t := env.With.Deferred
			if t.locked && t.pendingMergeRhs != nil && !t.fixPublish {
				w = mergeAttrBindMaps(t.memo, t.pendingMergeRhs)
			} else {
				w, err = ev.forceValue(env.With)
			}
		} else {
			w, err = ev.forceValue(env.With)
		}
		if err != nil {
			return nil, false
		}
		if w != nil && w.Kind == VKAttrs {
			if v, ok := getAttr(w, sym); ok {
				return &AttrBinding{Value: v}, true
			}
		}
	}
	return nil, false
}

func getAttr(v *Value, sym data.Symbol) (*Value, bool) {
	if v == nil || v.Kind != VKAttrs || v.Attrs == nil {
		return nil, false
	}
	b, ok := v.Attrs.Binds[sym]
	if !ok || b == nil {
		return nil, false
	}
	if b.Thunk != nil {
		return Deferred(b.Thunk), true
	}
	if b.Value != nil {
		return b.Value, true
	}
	return nil, false
}

// Resolve rewrites variable references using static scope information.
// Dynamic `with` scopes are preserved; let/rec/lambda bindings are resolved to
// de Bruijn-style local indices stored in parser.Var.Resolved.
func Resolve(expr parser.Expr, env *resolveEnv) (parser.Expr, error) {
	if env == nil {
		env = newResolveEnv(nil)
	}
	return resolveExpr(expr, env)
}

type resolveEnv struct {
	parent *resolveEnv
	vars   map[string]int // name -> binding depth
	depth  int
}

func newResolveEnv(parent *resolveEnv) *resolveEnv {
	return &resolveEnv{
		parent: parent,
		vars:   make(map[string]int),
		depth:  0,
	}
}

func (re *resolveEnv) child() *resolveEnv {
	return &resolveEnv{
		parent: re,
		vars:   make(map[string]int),
		depth:  re.depth + 1,
	}
}

func (re *resolveEnv) bind(name string) {
	re.vars[name] = re.depth
}

func (re *resolveEnv) lookup(name string) (int, bool) {
	for cur := re; cur != nil; cur = cur.parent {
		if d, ok := cur.vars[name]; ok {
			return d, true
		}
	}
	return 0, false
}

func resolveExpr(expr parser.Expr, env *resolveEnv) (parser.Expr, error) {
	if expr == nil {
		return nil, nil
	}
	switch e := expr.(type) {
	case *parser.Integer, *parser.Float, *parser.String, *parser.IndentedString,
		*parser.Path, *parser.URI, *parser.CurPos:
		return expr, nil
	case *parser.Var:
		if _, ok := env.lookup(e.Name); ok {
			return expr, nil
		}
		return expr, nil
	case *parser.List:
		out := &parser.List{P: e.P, Elems: make([]parser.Expr, len(e.Elems))}
		for i, el := range e.Elems {
			r, err := resolveExpr(el, env)
			if err != nil {
				return nil, err
			}
			out.Elems[i] = r
		}
		return out, nil
	case *parser.AttrSet:
		child := env.child()
		if e.Recursive {
			for _, b := range e.Binds {
				for _, seg := range b.Path {
					if seg.Expr == nil {
						child.bind(seg.Name)
					}
				}
			}
		}
		out := &parser.AttrSet{P: e.P, Recursive: e.Recursive}
		for _, b := range e.Binds {
			nb, err := resolveBinding(b, child, e.Recursive)
			if err != nil {
				return nil, err
			}
			out.Binds = append(out.Binds, nb)
		}
		for _, inh := range e.Inherit {
			out.Inherit = append(out.Inherit, inh)
		}
		return out, nil
	case *parser.Call:
		f, err := resolveExpr(e.Fun, env)
		if err != nil {
			return nil, err
		}
		out := &parser.Call{P: e.P, Fun: f, Args: make([]parser.Expr, len(e.Args))}
		for i, a := range e.Args {
			r, err := resolveExpr(a, env)
			if err != nil {
				return nil, err
			}
			out.Args[i] = r
		}
		return out, nil
	case *parser.Select:
		base, err := resolveExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		return &parser.Select{P: e.P, Expr: base, Path: e.Path}, nil
	case *parser.SelectOr:
		base, err := resolveExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		def, err := resolveExpr(e.Default, env)
		if err != nil {
			return nil, err
		}
		return &parser.SelectOr{P: e.P, Expr: base, Path: e.Path, Default: def}, nil
	case *parser.HasAttr:
		base, err := resolveExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		return &parser.HasAttr{P: e.P, Expr: base, Path: e.Path}, nil
	case *parser.UnaryOp:
		x, err := resolveExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		return &parser.UnaryOp{P: e.P, Op: e.Op, Expr: x}, nil
	case *parser.BinaryOp:
		l, err := resolveExpr(e.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := resolveExpr(e.Right, env)
		if err != nil {
			return nil, err
		}
		return &parser.BinaryOp{P: e.P, Op: e.Op, Left: l, Right: r}, nil
	case *parser.If:
		c, err := resolveExpr(e.Cond, env)
		if err != nil {
			return nil, err
		}
		t, err := resolveExpr(e.Then, env)
		if err != nil {
			return nil, err
		}
		el, err := resolveExpr(e.Else, env)
		if err != nil {
			return nil, err
		}
		return &parser.If{P: e.P, Cond: c, Then: t, Else: el}, nil
	case *parser.Assert:
		c, err := resolveExpr(e.Cond, env)
		if err != nil {
			return nil, err
		}
		b, err := resolveExpr(e.Body, env)
		if err != nil {
			return nil, err
		}
		return &parser.Assert{P: e.P, Cond: c, Body: b}, nil
	case *parser.With:
		attrs, err := resolveExpr(e.Attrs, env)
		if err != nil {
			return nil, err
		}
		child := env.child()
		b, err := resolveExpr(e.Body, child)
		if err != nil {
			return nil, err
		}
		return &parser.With{P: e.P, Attrs: attrs, Body: b}, nil
	case *parser.Let:
		child := env.child()
		if e.Attrs != nil && e.Attrs.Recursive {
			for _, b := range e.Attrs.Binds {
				for _, seg := range b.Path {
					if seg.Expr == nil {
						child.bind(seg.Name)
					}
				}
			}
		}
		attrs, err := resolveAttrSet(e.Attrs, child, true)
		if err != nil {
			return nil, err
		}
		body, err := resolveExpr(e.Body, child)
		if err != nil {
			return nil, err
		}
		return &parser.Let{P: e.P, Attrs: attrs, Body: body}, nil
	case *parser.Lambda:
		child := env.child()
		if e.Arg != "" {
			child.bind(e.Arg)
		}
		if e.Formals != nil {
			for _, f := range e.Formals.Params {
				child.bind(f.Name)
			}
		}
		body, err := resolveExpr(e.Body, child)
		if err != nil {
			return nil, err
		}
		return &parser.Lambda{
			P: e.P, Arg: e.Arg, Formals: e.Formals, Body: body, ArgFirst: e.ArgFirst,
		}, nil
	case *parser.Parens:
		x, err := resolveExpr(e.Expr, env)
		if err != nil {
			return nil, err
		}
		return &parser.Parens{P: e.P, Expr: x}, nil
	case *parser.ConcatStrings:
		out := &parser.ConcatStrings{P: e.P, Parts: make([]parser.Expr, len(e.Parts))}
		for i, p := range e.Parts {
			r, err := resolveExpr(p, env)
			if err != nil {
				return nil, err
			}
			out.Parts[i] = r
		}
		return out, nil
	default:
		return expr, nil
	}
}

func resolveBinding(b parser.Binding, env *resolveEnv, rec bool) (parser.Binding, error) {
	if !rec {
		for _, seg := range b.Path {
			if seg.Expr == nil {
				env.bind(seg.Name)
			}
		}
	}
	val, err := resolveExpr(b.Value, env)
	if err != nil {
		return parser.Binding{}, err
	}
	return parser.Binding{Pos: b.Pos, Path: b.Path, Value: val, Kind: b.Kind}, nil
}

func resolveAttrSet(a *parser.AttrSet, env *resolveEnv, isLet bool) (*parser.AttrSet, error) {
	if a == nil {
		return nil, nil
	}
	child := env
	if !isLet {
		child = env.child()
		if a.Recursive {
			for _, b := range a.Binds {
				for _, seg := range b.Path {
					if seg.Expr == nil {
						child.bind(seg.Name)
					}
				}
			}
		}
	}
	out := &parser.AttrSet{P: a.P, Recursive: a.Recursive}
	for _, b := range a.Binds {
		nb, err := resolveBinding(b, child, a.Recursive)
		if err != nil {
			return nil, err
		}
		out.Binds = append(out.Binds, nb)
	}
	out.Inherit = a.Inherit
	return out, nil
}
