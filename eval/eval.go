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
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// EvalError reports evaluation failures with source context.
type EvalError struct {
	Pos parser.Pos
	Msg string
}

func (e *EvalError) Error() string {
	if e.Pos.File != "" {
		return fmt.Sprintf("%s:%d:%d: %s", e.Pos.File, e.Pos.Line, e.Pos.Col, e.Msg)
	}
	return e.Msg
}

func evalErr(pos parser.Pos, msg string, args ...any) error {
	return &EvalError{Pos: pos, Msg: fmt.Sprintf(msg, args...)}
}

// Evaluator walks the parser AST with lazy semantics.
type Evaluator struct {
	Intern   *data.Interner
	Effects  Effects
	Builtins map[string]*Builtin
	NixPath  *NixPath
	StoreCfg storeConfig

	curFile     string
	importDepth int
	importCache map[string]*Value

	forceTrace  bool
	forceFrames []string

	drvStrictByPath map[string]*DerivationValue
	strictDrvs      map[string]*gonixdrv.Derivation // every derivationStrict result, keyed by drvPath
	moduloCache     *store.ModuloCache
}

type storeConfig struct {
	storeDir string
}

// NewEvaluator constructs an evaluator with default builtins and pure effects.
func NewEvaluator() *Evaluator {
	intern := data.NewInterner()
	e := &Evaluator{
		Intern:      intern,
		Effects:     PureEffects{},
		Builtins:    make(map[string]*Builtin),
		NixPath:     NewNixPathFromEnv(),
		importCache: make(map[string]*Value),
	}
	e.registerBuiltins()
	if os.Getenv("GOONIX_TRACE_FORCE") != "" {
		e.forceTrace = true
	}
	return e
}

// WithEffects returns a copy using the given effects implementation.
func (ev *Evaluator) WithEffects(eff Effects) *Evaluator {
	c := *ev
	c.Effects = eff
	if eff != nil {
		c.StoreCfg.storeDir = eff.StoreDir()
	}
	if c.importCache == nil {
		c.importCache = make(map[string]*Value)
	} else {
		cache := make(map[string]*Value, len(ev.importCache))
		for k, v := range ev.importCache {
			cache[k] = v
		}
		c.importCache = cache
	}
	return &c
}

// Eval evaluates expr in a fresh environment and forces the result.
func (ev *Evaluator) Eval(expr parser.Expr) (*Value, error) {
	v, err := ev.evalExpr(expr, NewEnv())
	if err != nil {
		return nil, err
	}
	return ev.forceValue(v)
}

// SetCurFile sets the path of the expression being evaluated (for relative imports).
func (ev *Evaluator) SetCurFile(path string) {
	ev.curFile = path
}

// EvalWithBuiltins evaluates expr with `builtins` in scope.
func (ev *Evaluator) EvalWithBuiltins(expr parser.Expr) (*Value, error) {
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	v, err := ev.evalExpr(expr, env)
	if err != nil {
		return nil, err
	}
	return ev.forceValue(v)
}

// ForceValue forces thunks (currently a no-op on plain values).
func (ev *Evaluator) ForceValue(v *Value) (*Value, error) {
	return ev.forceValue(v)
}

// EvalString parses and evaluates source text.
func (ev *Evaluator) EvalString(src string) (*Value, error) {
	expr, err := parser.ParseString(src)
	if err != nil {
		return nil, err
	}
	return ev.Eval(expr)
}

func (ev *Evaluator) evalExpr(expr parser.Expr, env *Env) (*Value, error) {
	if expr == nil {
		return Null(), nil
	}
	if p := expr.Pos(); p.File != "" {
		ev.curFile = p.File
	}
	switch e := expr.(type) {
	case *parser.Integer:
		i, err := strconv.ParseInt(e.Text, 0, 64)
		if err != nil {
			return nil, evalErr(e.P, "invalid integer: %v", err)
		}
		return Int(i), nil
	case *parser.Float:
		f, err := strconv.ParseFloat(e.Text, 64)
		if err != nil {
			return nil, evalErr(e.P, "invalid float: %v", err)
		}
		return Float(f), nil
	case *parser.Var:
		return ev.evalVar(e, env)
	case *parser.String:
		return ev.evalString(e, env)
	case *parser.IndentedString:
		return ev.evalIndentedString(e, env)
	case *parser.Path:
		return ev.evalPath(e, env)
	case *parser.URI:
		return Str(e.Text, e.P), nil
	case *parser.List:
		out := make([]*Value, len(e.Elems))
		for i, el := range e.Elems {
			t := ev.newThunk(el, env, el.Pos())
			out[i] = Deferred(t)
		}
		return List(out...), nil
	case *parser.AttrSet:
		return ev.evalAttrSet(e, env, false)
	case *parser.Call:
		return ev.evalCall(e, env)
	case *parser.Select:
		return ev.evalSelect(e, env, false, nil)
	case *parser.SelectOr:
		v, err := ev.evalSelect(&parser.Select{P: e.P, Expr: e.Expr, Path: e.Path}, env, true, e.Default)
		return v, err
	case *parser.HasAttr:
		return ev.evalHasAttr(e, env)
	case *parser.UnaryOp:
		return ev.evalUnary(e, env)
	case *parser.BinaryOp:
		return ev.evalBinary(e, env)
	case *parser.If:
		cv, err := ev.evalExpr(e.Cond, env)
		if err != nil {
			return nil, err
		}
		c, err := ev.forceBool(ev, cv, env)
		if err != nil {
			return nil, err
		}
		if c {
			return ev.evalExpr(e.Then, env)
		}
		return ev.evalExpr(e.Else, env)
	case *parser.Assert:
		cv, err := ev.evalExpr(e.Cond, env)
		if err != nil {
			return nil, err
		}
		c, err := ev.forceBool(ev, cv, env)
		if err != nil {
			return nil, err
		}
		if !c {
			return nil, evalErr(e.P, "assertion failed")
		}
		return ev.evalExpr(e.Body, env)
	case *parser.With:
		child := env.extend()
		t := ev.newThunk(e.Attrs, env, e.P)
		child.With = Deferred(t)
		return ev.evalExpr(e.Body, child)
	case *parser.Let:
		return ev.evalLet(e, env)
	case *parser.Lambda:
		return &Value{
			Kind: VKLambda,
			Lambda: &LambdaValue{
				SimpleArg: e.Arg,
				Formals:   e.Formals,
				ArgFirst:  e.ArgFirst,
				Body:      e.Body,
				Env:       env,
			},
		}, nil
	case *parser.Parens:
		return ev.evalExpr(e.Expr, env)
	case *parser.ConcatStrings:
		return ev.evalConcatStrings(e, env)
	case *parser.CurPos:
		return Str(fmt.Sprintf("%s:%d:%d", e.P.File, e.P.Line, e.P.Col), e.P), nil
	default:
		return nil, evalErr(expr.Pos(), "unsupported expression %T", expr)
	}
}

func (ev *Evaluator) evalVar(v *parser.Var, env *Env) (*Value, error) {
	if v.Name == "true" {
		return Bool(true), nil
	}
	if v.Name == "false" {
		return Bool(false), nil
	}
	if v.Name == "null" {
		return Null(), nil
	}
	sym := ev.Intern.Intern(v.Name)
	if bind, ok, err := ev.lookupEnv(env, sym); err != nil {
		return nil, err
	} else if ok {
		if bind.Thunk != nil {
			return Deferred(bind.Thunk), nil
		}
		return bind.Value, nil
	}
	if b, ok := ev.Builtins[v.Name]; ok {
		return &Value{Kind: VKBuiltin, Builtin: b}, nil
	}
	return nil, evalErr(v.P, "undefined variable %q", v.Name)
}

func (ev *Evaluator) evalString(s *parser.String, env *Env) (*Value, error) {
	var b strings.Builder
	var ctx []StringContextElem
	cc := CoerceContext{Mode: CoerceString, Pos: s.P}
	for _, part := range s.Parts {
		if part.Interp == nil {
			b.WriteString(part.Text)
			continue
		}
		v, err := ev.evalExpr(part.Interp, env)
		if err != nil {
			return nil, err
		}
		sv, err := ev.coerceToString(v, cc)
		if err != nil {
			return nil, err
		}
		b.WriteString(sv.String)
		ctx = mergeStringContexts(ctx, sv.Ctx)
	}
	return StrWithContext(b.String(), ctx, s.P), nil
}

func (ev *Evaluator) evalIndentedString(s *parser.IndentedString, env *Env) (*Value, error) {
	var b strings.Builder
	var ctx []StringContextElem
	cc := CoerceContext{Mode: CoerceString, Pos: s.P}
	for _, part := range s.Parts {
		if part.Interp == nil {
			b.WriteString(part.Text)
			continue
		}
		v, err := ev.evalExpr(part.Interp, env)
		if err != nil {
			return nil, err
		}
		sv, err := ev.coerceToString(v, cc)
		if err != nil {
			return nil, err
		}
		b.WriteString(sv.String)
		ctx = mergeStringContexts(ctx, sv.Ctx)
	}
	return StrWithContext(b.String(), ctx, s.P), nil
}

func (ev *Evaluator) evalPath(p *parser.Path, env *Env) (*Value, error) {
	text := p.Text
	if strings.HasPrefix(text, "<") && strings.HasSuffix(text, ">") {
		name := strings.TrimSuffix(strings.TrimPrefix(text, "<"), ">")
		resolved, err := ev.NixPath.FindFile(name)
		if err != nil {
			return nil, evalErr(p.P, "%v", err)
		}
		return Path(resolved), nil
	}
	if strings.HasPrefix(text, "./") || strings.HasPrefix(text, "../") {
		if ev.curFile != "" {
			text = filepath.Join(filepath.Dir(ev.curFile), text)
		}
	}
	if strings.HasPrefix(text, "./") || strings.HasPrefix(text, "../") || strings.HasPrefix(text, "/") {
		return Path(text), nil
	}
	// Path with interpolations is represented via ConcatStrings in parser; bare paths here.
	return Path(text), nil
}

func (ev *Evaluator) evalAttrSet(a *parser.AttrSet, env *Env, fromLet bool) (*Value, error) {
	_ = fromLet
	av := Attrs(a.Recursive)
	if err := ev.populateAttrSet(a, env, av); err != nil {
		return nil, err
	}
	return av, nil
}

func (ev *Evaluator) populateAttrSet(a *parser.AttrSet, env *Env, av *Value) error {
	bindEnv := env.extend()

	applyInherit := func(inh parser.Inherit) error {
		return ev.bindAttrInherit(inh, env, bindEnv, av, a.Recursive)
	}
	applyBind := func(bind parser.Binding) error {
		return ev.insertBinding(av, bindEnv, bind, a.Recursive)
	}

	if len(a.Members) > 0 {
		for _, m := range a.Members {
			if m.Inherit != nil {
				if err := applyInherit(*m.Inherit); err != nil {
					return err
				}
			}
			if m.Binding != nil {
				if err := applyBind(*m.Binding); err != nil {
					return err
				}
			}
		}
	} else {
		for _, bind := range a.Binds {
			if err := applyBind(bind); err != nil {
				return err
			}
		}
		for _, inh := range a.Inherit {
			if err := applyInherit(inh); err != nil {
				return err
			}
		}
	}

	if a.Recursive {
		ev.syncRecTopLevel(av, bindEnv)
	}
	return nil
}

func (ev *Evaluator) bindAttrInherit(inh parser.Inherit, populateEnv, bindEnv *Env, av *Value, recursive bool) error {
	for _, n := range inh.Names {
		name := n.Name
		if n.Expr != nil {
			continue
		}
		sym := ev.Intern.Intern(name)
		var expr parser.Expr
		if inh.From != nil {
			expr = &parser.Select{
				P:    inh.Pos,
				Expr: inh.From,
				Path: []parser.AttrName{{Name: name}},
			}
		} else {
			expr = &parser.Var{P: inh.Pos, Name: name}
		}
		lookupEnv := populateEnv
		if inh.From != nil && recursive {
			lookupEnv = bindEnv
		}
		t := ev.newThunk(expr, lookupEnv, inh.Pos)
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		if inh.From != nil && recursive {
			bindEnv.bindThunk(sym, t)
		}
	}
	return nil
}


func (ev *Evaluator) bindingSym(b parser.Binding) data.Symbol {
	if len(b.Path) != 1 || b.Path[0].Expr != nil {
		return 0
	}
	return ev.Intern.Intern(b.Path[0].Name)
}

func (ev *Evaluator) evalInherit(inh parser.Inherit, env *Env, av *Value) error {
	var from *Value
	if inh.From != nil {
		v, err := ev.evalExpr(inh.From, env)
		if err != nil {
			return err
		}
		fv, err := ev.forceValue(v)
		if err != nil {
			return err
		}
		if fv.Kind != VKAttrs {
			return evalErr(inh.Pos, "inherit from expects attribute set")
		}
		from = fv
	}
	for _, n := range inh.Names {
		name := n.Name
		if n.Expr != nil {
			v, err := ev.evalExpr(n.Expr, env)
			if err != nil {
				return err
			}
			fv, err := ev.forceString(v)
			if err != nil {
				return err
			}
			name = fv
		}
		sym := ev.Intern.Intern(name)
		var val *Value
		if from != nil {
			if b, ok := from.Attrs.Binds[sym]; ok && b != nil {
				if b.Thunk != nil {
					var err error
					val, err = ev.forceThunk(b.Thunk)
					if err != nil {
						return err
					}
				} else {
					val = b.Value
				}
			} else {
				return evalErr(inh.Pos, "attribute %q missing", name)
			}
		} else {
			if bind, ok, err := ev.lookupEnv(env, sym); err != nil {
				return err
			} else if ok {
				if bind.Thunk != nil {
					var err error
					val, err = ev.forceThunk(bind.Thunk)
					if err != nil {
						return err
					}
				} else {
					val = bind.Value
				}
			} else {
				return evalErr(inh.Pos, "undefined variable %q in inherit", name)
			}
		}
		av.Attrs.Binds[sym] = &AttrBinding{Value: val}
	}
	return nil
}

func (ev *Evaluator) evalLet(l *parser.Let, env *Env) (*Value, error) {
	child := env.extend()
	if l.Attrs != nil {
		seen := make(map[data.Symbol]struct{})
		bindOne := func(bind parser.Binding) {
			sym := ev.bindingSym(bind)
			if sym == 0 {
				return
			}
			if _, ok := seen[sym]; ok {
				return
			}
			seen[sym] = struct{}{}
			t := ev.newThunk(bind.Value, child, bind.Pos)
			child.bindThunk(sym, t)
		}
		if len(l.Attrs.Members) > 0 {
			for _, m := range l.Attrs.Members {
				if m.Inherit != nil {
					if err := ev.bindLetInherit(*m.Inherit, child); err != nil {
						return nil, err
					}
				}
				if m.Binding != nil {
					bindOne(*m.Binding)
				}
			}
		} else {
			for _, inh := range l.Attrs.Inherit {
				if err := ev.bindLetInherit(inh, child); err != nil {
					return nil, err
				}
			}
		}
		for _, bind := range l.Attrs.Binds {
			bindOne(bind)
		}
	}
	return ev.evalExpr(l.Body, child)
}

func (ev *Evaluator) bindLetInherit(inh parser.Inherit, env *Env) error {
	for _, n := range inh.Names {
		name := n.Name
		if n.Expr != nil {
			// inherit (from) ${name} — handled via dynamic name in inherit from set
			continue
		}
		sym := ev.Intern.Intern(name)
		var expr parser.Expr
		if inh.From != nil {
			expr = &parser.Select{
				P:    inh.Pos,
				Expr: inh.From,
				Path: []parser.AttrName{{Name: name}},
			}
		} else {
			expr = &parser.Var{P: inh.Pos, Name: name}
		}
		t := ev.newThunk(expr, env, inh.Pos)
		env.bindThunk(sym, t)
	}
	return nil
}

func (ev *Evaluator) evalCall(c *parser.Call, env *Env) (*Value, error) {
	if len(c.Args) >= 1 {
		if orv, ok := c.Args[0].(*parser.Var); ok && orv.Name == "or" {
			return ev.evalOrFallback(c.Fun, c.Args[1:], env, c.P)
		}
	}
	fun, err := ev.evalExpr(c.Fun, env)
	if err != nil {
		return nil, err
	}
	ff, err := ev.mustWHNF(fun, c.P)
	if err != nil {
		return nil, err
	}
	if !ev.isCallableValue(ff, c.P) {
		ev.logTypeMismatchWant(c.P, ff, "function")
		return nil, evalErr(c.P, "attempt to call non-function")
	}
	args := make([]*Value, len(c.Args))
	for i, a := range c.Args {
		t := ev.newThunk(a, env, a.Pos())
		args[i] = Deferred(t)
	}
	if ev.isCallableValue(ff, c.P) {
		return ev.applyCallableN(ff, args, c.P)
	}
	return nil, evalErr(c.P, "attempt to call non-function")
}

// evalOrFallback implements `e1 or e2` (cursed invoke): if e1 is null, e2; else e1 without forcing thunks.
func (ev *Evaluator) evalOrFallback(base parser.Expr, rest []parser.Expr, env *Env, pos parser.Pos) (*Value, error) {
	baseVal, err := ev.evalExpr(base, env)
	if err != nil {
		return nil, err
	}
	if baseVal != nil && baseVal.Kind == VKDeferred {
		return baseVal, nil
	}
	fb, err := ev.forceWHNF(baseVal, pos)
	if err != nil {
		return nil, err
	}
	if fb != nil && fb.Kind == VKNull {
		if len(rest) == 0 {
			return Null(), nil
		}
		return ev.evalExpr(rest[0], env)
	}
	return fb, nil
}

func (ev *Evaluator) applyLambda(l *LambdaValue, args []*Value, pos parser.Pos) (*Value, error) {
	if l == nil {
		return nil, evalErr(pos, "invalid lambda")
	}
	child := l.Env.extend()
	if l.Formals == nil && l.SimpleArg != "" {
		if len(args) != 1 {
			return nil, evalErr(pos, "function expects 1 argument, got %d", len(args))
		}
		sym := ev.Intern.Intern(l.SimpleArg)
		child.bind(sym, args[0])
		return ev.evalExpr(l.Body, child)
	}
	if l.Formals != nil {
		return ev.applyFormals(l, args, child, pos)
	}
	return nil, evalErr(pos, "invalid lambda formals")
}

func (ev *Evaluator) applyFormals(l *LambdaValue, args []*Value, child *Env, pos parser.Pos) (*Value, error) {
	named := make(map[string]*Value)
	var rest []*Value
	if len(args) == 1 {
		fa, err := ev.forceAttrs(args[0], pos)
		if err != nil {
			return nil, err
		}
		if l.SimpleArg != "" {
			if l.ArgFirst {
				child.bind(ev.Intern.Intern(l.SimpleArg), fa)
			} else {
				child.bind(ev.Intern.Intern(l.SimpleArg), args[0])
			}
		}
		if l.Formals != nil {
			for _, f := range l.Formals.Params {
				formalSym := ev.Intern.Intern(f.Name)
				if b, ok := fa.Attrs.Binds[formalSym]; ok {
					if b.Thunk != nil {
						child.bindThunk(formalSym, b.Thunk)
					} else {
						child.bind(formalSym, b.Value)
					}
					continue
				}
				if f.Default != nil {
					child.bindThunk(formalSym, ev.newThunk(f.Default, child, pos))
					continue
				}
				return nil, evalErr(pos, "argument %q missing", f.Name)
			}
			if l.Formals.Ellipsis {
				// Rest formal: pass the full attribute set (WHNF head only).
				rest = []*Value{args[0]}
			}
		}
	} else {
		rest = args
	}
	if l.Formals == nil {
		return ev.evalExpr(l.Body, child)
	}
	for _, f := range l.Formals.Params {
		formalSym := ev.Intern.Intern(f.Name)
		if _, ok := child.lookup(formalSym); ok {
			continue
		}
		if v, ok := named[f.Name]; ok {
			child.bind(formalSym, v)
			delete(named, f.Name)
			continue
		}
		if f.Default != nil {
			child.bindThunk(formalSym, ev.newThunk(f.Default, child, pos))
			continue
		}
		if len(rest) > 0 {
			child.bind(formalSym, rest[0])
			rest = rest[1:]
			continue
		}
		return nil, evalErr(pos, "argument %q missing", f.Name)
	}
	if !l.Formals.Ellipsis && (len(named) > 0 || len(rest) > 0) {
		return nil, evalErr(pos, "too many arguments")
	}
	return ev.evalExpr(l.Body, child)
}

func (ev *Evaluator) evalSelect(s *parser.Select, env *Env, hasDefault bool, defaultExpr parser.Expr) (*Value, error) {
	base, err := ev.evalExpr(s.Expr, env)
	if err != nil {
		return nil, err
	}
	var fb *Value
	if hasDefault {
		fb, err = ev.forceWHNF(base, s.P)
	} else {
		fb, err = ev.mustWHNF(base, s.P)
	}
	if err != nil {
		return nil, err
	}
	for i, seg := range s.Path {
		name := seg.Name
		if seg.Expr != nil {
			nv, err := ev.evalExpr(seg.Expr, env)
			if err != nil {
				return nil, err
			}
			ns, err := ev.forceString(nv)
			if err != nil {
				return nil, err
			}
			name = ns
		}
		sym := ev.Intern.Intern(name)
		if fb.Kind != VKAttrs {
			if hasDefault {
				return ev.evalExpr(defaultExpr, env)
			}
			ev.logTypeMismatchWant(s.P, fb, "attribute set")
			return nil, evalErr(s.P, "value is not an attribute set")
		}
		if fb.Kind == VKAttrs {
			b, ok := fb.Attrs.Binds[sym]
			if !ok {
				if hasDefault {
					return ev.evalExpr(defaultExpr, env)
				}
				if fb.Attrs.Blackhole {
					return blackholeAttrs(), nil
				}
				return nil, evalErr(s.P, "attribute %q missing", name)
			}
			if i == len(s.Path)-1 {
				if b.Thunk != nil {
					fb = Deferred(b.Thunk)
				} else {
					fb = b.Value
				}
				break
			}
			var next *Value
			if b.Thunk != nil {
				next, err = ev.mustWHNF(Deferred(b.Thunk), s.P)
			} else {
				next, err = ev.mustWHNF(b.Value, s.P)
			}
			if err != nil {
				return nil, err
			}
			fb = next
			continue
		}
	}
	if fb.Kind == VKBuiltin && fb.Builtin != nil && fb.Builtin.Nullary {
		return fb.Builtin.Fn(ev, s.P, nil)
	}
	return fb, nil
}

func (ev *Evaluator) evalHasAttr(h *parser.HasAttr, env *Env) (*Value, error) {
	base, err := ev.evalExpr(h.Expr, env)
	if err != nil {
		return nil, err
	}
	fb, err := ev.forceWHNF(base, h.P)
	if err != nil {
		return nil, err
	}
	if fb.Kind != VKAttrs {
		return Bool(false), nil
	}
	for i, seg := range h.Path {
		name := seg.Name
		if seg.Expr != nil {
			nv, err := ev.evalExpr(seg.Expr, env)
			if err != nil {
				return nil, err
			}
			ns, err := ev.forceString(nv)
			if err != nil {
				return nil, err
			}
			name = ns
		}
		sym := ev.Intern.Intern(name)
		b, ok := fb.Attrs.Binds[sym]
		if !ok {
			return Bool(false), nil
		}
		if i == len(h.Path)-1 {
			return Bool(true), nil
		}
		if b.Thunk != nil {
			fb, err = ev.forceWHNF(Deferred(b.Thunk), h.P)
		} else {
			fb, err = ev.forceWHNF(b.Value, h.P)
		}
		if err != nil {
			return Bool(false), nil
		}
		if fb.Kind != VKAttrs {
			return Bool(false), nil
		}
	}
	return Bool(true), nil
}

func (ev *Evaluator) evalUnary(u *parser.UnaryOp, env *Env) (*Value, error) {
	v, err := ev.evalExpr(u.Expr, env)
	if err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(v, u.P)
	if err != nil {
		return nil, err
	}
	switch u.Op {
	case "-":
		switch fv.Kind {
		case VKInt:
			return Int(-fv.Int), nil
		case VKFloat:
			return Float(-fv.Float), nil
		}
	case "!":
		b, err := ev.valueToBool(fv)
		if err != nil {
			return nil, err
		}
		return Bool(!b), nil
	}
	return nil, evalErr(u.P, "invalid unary operator %q", u.Op)
}

func (ev *Evaluator) evalBinary(b *parser.BinaryOp, env *Env) (*Value, error) {
	switch b.Op {
	case "||":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := ev.forceBool(ev, l, env)
		if err != nil {
			return nil, err
		}
		if lb {
			return Bool(true), nil
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := ev.forceBool(ev, r, env)
		if err != nil {
			return nil, err
		}
		return Bool(rb), nil
	case "&&":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := ev.forceBool(ev, l, env)
		if err != nil {
			return nil, err
		}
		if !lb {
			return Bool(false), nil
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := ev.forceBool(ev, r, env)
		if err != nil {
			return nil, err
		}
		return Bool(rb), nil
	case "->":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		lb, err := ev.forceBool(ev, l, env)
		if err != nil {
			return nil, err
		}
		if !lb {
			return Bool(true), nil
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		rb, err := ev.forceBool(ev, r, env)
		if err != nil {
			return nil, err
		}
		return Bool(rb), nil
	case "++":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		return ev.concatValues(l, r, b.P)
	case "//":
		return ev.evalOpUpdate(b, env)
	case "==", "!=":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		eq, err := ev.equalValues(l, r)
		if err != nil {
			return nil, err
		}
		if b.Op == "!=" {
			eq = !eq
		}
		return Bool(eq), nil
	case "+", "-", "*", "/":
		return ev.evalArithmetic(b, env)
	case "<", "<=", ">", ">=":
		return ev.evalCompare(b, env)
	}
	return nil, evalErr(b.P, "unknown operator %q", b.Op)
}

func (ev *Evaluator) evalArithmetic(b *parser.BinaryOp, env *Env) (*Value, error) {
	l, err := ev.evalExpr(b.Left, env)
	if err != nil {
		return nil, err
	}
	r, err := ev.evalExpr(b.Right, env)
	if err != nil {
		return nil, err
	}
	fl, err := ev.mustWHNF(l, b.P)
	if err != nil {
		return nil, err
	}
	fr, err := ev.mustWHNF(r, b.P)
	if err != nil {
		return nil, err
	}
	if b.Op == "+" && (fl.Kind == VKString || fr.Kind == VKString || fl.Kind == VKPath || fr.Kind == VKPath) {
		return ev.concatCoercedStrings(fl, fr, b.P)
	}
	if fl.Kind == VKInt && fr.Kind == VKInt {
		switch b.Op {
		case "+":
			return Int(fl.Int + fr.Int), nil
		case "-":
			return Int(fl.Int - fr.Int), nil
		case "*":
			return Int(fl.Int * fr.Int), nil
		case "/":
			if fr.Int == 0 {
				return nil, evalErr(b.P, "division by zero")
			}
			return Int(fl.Int / fr.Int), nil
		}
	}
	if (fl.Kind == VKInt || fl.Kind == VKFloat) && (fr.Kind == VKInt || fr.Kind == VKFloat) {
		lf := float64(fl.Int)
		if fl.Kind == VKFloat {
			lf = fl.Float
		}
		rf := float64(fr.Int)
		if fr.Kind == VKFloat {
			rf = fr.Float
		}
		var out float64
		switch b.Op {
		case "+":
			out = lf + rf
		case "-":
			out = lf - rf
		case "*":
			out = lf * rf
		case "/":
			if rf == 0 {
				return nil, evalErr(b.P, "division by zero")
			}
			out = lf / rf
		}
		return Float(out), nil
	}
	return nil, evalErr(b.P, "invalid arithmetic operands")
}

func (ev *Evaluator) evalCompare(b *parser.BinaryOp, env *Env) (*Value, error) {
	l, err := ev.evalExpr(b.Left, env)
	if err != nil {
		return nil, err
	}
	r, err := ev.evalExpr(b.Right, env)
	if err != nil {
		return nil, err
	}
	fl, err := ev.forceValue(l)
	if err != nil {
		return nil, err
	}
	fr, err := ev.forceValue(r)
	if err != nil {
		return nil, err
	}
	ok := false
	if fl.Kind == VKInt && fr.Kind == VKInt {
		switch b.Op {
		case "<":
			ok = fl.Int < fr.Int
		case "<=":
			ok = fl.Int <= fr.Int
		case ">":
			ok = fl.Int > fr.Int
		case ">=":
			ok = fl.Int >= fr.Int
		}
		return Bool(ok), nil
	}
	if fl.Kind == VKFloat || fr.Kind == VKFloat || fl.Kind == VKInt || fr.Kind == VKInt {
		lf := float64(fl.Int)
		if fl.Kind == VKFloat {
			lf = fl.Float
		}
		rf := float64(fr.Int)
		if fr.Kind == VKFloat {
			rf = fr.Float
		}
		switch b.Op {
		case "<":
			ok = lf < rf
		case "<=":
			ok = lf <= rf
		case ">":
			ok = lf > rf
		case ">=":
			ok = lf >= rf
		}
		return Bool(ok), nil
	}
	return nil, evalErr(b.P, "invalid comparison operands")
}

func (ev *Evaluator) evalConcatStrings(c *parser.ConcatStrings, env *Env) (*Value, error) {
	if len(c.Parts) == 0 {
		return Str("", c.P), nil
	}
	vals := make([]*Value, len(c.Parts))
	for i, p := range c.Parts {
		v, err := ev.evalExpr(p, env)
		if err != nil {
			return nil, err
		}
		fv, err := ev.forceValue(v)
		if err != nil {
			return nil, err
		}
		vals[i] = fv
	}
	anyString := false
	allInt := true
	for _, fv := range vals {
		if fv.Kind == VKString || fv.Kind == VKPath {
			anyString = true
			allInt = false
		} else if fv.Kind != VKInt {
			allInt = false
		}
	}
	if !anyString && allInt && len(vals) >= 2 {
		var sum int64
		for _, fv := range vals {
			sum += fv.Int
		}
		return Int(sum), nil
	}
	var b strings.Builder
	var ctx []StringContextElem
	cc := CoerceContext{Mode: CoerceString, Pos: c.P}
	for _, fv := range vals {
		sv, err := ev.coerceToString(fv, cc)
		if err != nil {
			return nil, err
		}
		b.WriteString(sv.String)
		ctx = mergeStringContexts(ctx, sv.Ctx)
	}
	return StrWithContext(b.String(), ctx, c.P), nil
}

func (ev *Evaluator) evalOpUpdate(b *parser.BinaryOp, env *Env) (*Value, error) {
	l, err := ev.evalAttrs(b.Left, env, b.P)
	if err != nil {
		return nil, err
	}
	r, err := ev.evalAttrs(b.Right, env, b.P)
	if err != nil {
		return nil, err
	}
	return mergeAttrBindMaps(l, r), nil
}

func (ev *Evaluator) newThunk(expr parser.Expr, env *Env, pos parser.Pos) *Thunk {
	if pos.File == "" && expr != nil {
		pos = expr.Pos()
	}
	return &Thunk{eval: ev, expr: expr, env: env, pos: pos}
}

// deferredApplyCall returns a lazy list element / map result without applying f yet (Nix map laziness).
func (ev *Evaluator) deferredApplyCall(f, arg *Value, pos parser.Pos) *Value {
	fCap, argCap := f, arg
	posCap := pos
	t := &Thunk{
		eval: ev,
		pos:  posCap,
		lazy: func(ev *Evaluator) (*Value, error) {
			return ev.applyCallable(fCap, argCap, posCap)
		},
	}
	return Deferred(t)
}

func (ev *Evaluator) forceThunk(t *Thunk) (*Value, error) {
	if t == nil {
		return Null(), nil
	}
	if ev.forceTrace {
		ev.forceFrames = append(ev.forceFrames, ev.describeThunk(t))
		defer func() { ev.forceFrames = ev.forceFrames[:len(ev.forceFrames)-1] }()
	}
	if t.memo != nil && !t.locked {
		return t.memo, nil
	}
	if t.locked {
		msg := fmt.Sprintf("infinite recursion encountered at %s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
		if ev.forceTrace && len(ev.forceFrames) > 0 {
			msg += "\nforce stack (innermost last):\n  " + strings.Join(ev.forceFrames, "\n  ")
		}
		return nil, evalErr(t.pos, "%s", msg)
	}
	t.locked = true
	defer func() {
		t.locked = false
	}()
	var v *Value
	var err error
	if t.lazy != nil {
		v, err = t.lazy(ev)
	} else {
		v, err = ev.evalExpr(t.expr, t.env)
	}
	if err != nil {
		if t.errCtx != "" {
			if ee, ok := err.(*EvalError); ok {
				return nil, &EvalError{Pos: ee.Pos, Msg: t.errCtx + " " + ee.Msg}
			}
			return nil, fmt.Errorf("%s: %w", t.errCtx, err)
		}
		return nil, err
	}
	if v != nil && v.Kind == VKDeferred && v.Deferred == t {
		return nil, evalErr(t.pos, "infinite recursion encountered at %s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
	}
	t.memo = v
	return v, nil
}

func (ev *Evaluator) forceValue(v *Value) (*Value, error) {
	if v == nil {
		return Null(), nil
	}
	seen := make(map[*Thunk]struct{})
	for v != nil && v.Kind == VKDeferred && v.Deferred != nil {
		t := v.Deferred
		if t.locked {
			msg := fmt.Sprintf("infinite recursion encountered at %s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
			if ev.forceTrace && len(ev.forceFrames) > 0 {
				msg += "\nforce stack (innermost last):\n  " + strings.Join(ev.forceFrames, "\n  ")
			}
			return nil, evalErr(t.pos, "%s", msg)
		}
		if _, ok := seen[t]; ok {
			return nil, evalErr(t.pos, "infinite recursion encountered at %s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
		}
		seen[t] = struct{}{}
		if t.memo != nil {
			v = t.memo
			continue
		}
		var err error
		v, err = ev.forceThunk(t)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

// forceValueDeep is forceWHNF (legacy name for call sites).
func (ev *Evaluator) forceValueDeep(v *Value) (*Value, error) {
	return ev.mustWHNF(v, parser.Pos{})
}

func (ev *Evaluator) forceBool(e *Evaluator, v *Value, env *Env) (bool, error) {
	if v == nil {
		return false, nil
	}
	pos := parser.Pos{}
	if v.Kind == VKDeferred && v.Deferred != nil {
		pos = v.Deferred.pos
	}
	fv, err := ev.mustWHNF(v, pos)
	if err != nil {
		return false, err
	}
	return ev.valueToBool(fv)
}

func (ev *Evaluator) valueToBool(v *Value) (bool, error) {
	switch v.Kind {
	case VKBool:
		return v.Bool, nil
	case VKNull:
		return false, nil
	case VKInt:
		return v.Int != 0, nil
	case VKString:
		return v.String != "", nil
	case VKList:
		return len(v.List) > 0, nil
	case VKAttrs:
		return len(v.Attrs.Binds) > 0, nil
	default:
		return true, nil
	}
}

func (ev *Evaluator) forceString(v *Value) (string, error) {
	pos := parser.Pos{}
	if v != nil && v.Pos.File != "" {
		pos = v.Pos
	}
	if v != nil && v.Kind == VKDeferred && v.Deferred != nil && pos.File == "" {
		pos = v.Deferred.pos
	}
	sv, err := ev.coerceToString(v, CoerceContext{Mode: CoerceString, Pos: pos})
	if err != nil {
		return "", err
	}
	return sv.String, nil
}

func (ev *Evaluator) concatValues(a, b *Value, pos parser.Pos) (*Value, error) {
	fa, err := ev.forceValue(a)
	if err != nil {
		return nil, err
	}
	fb, err := ev.forceValue(b)
	if err != nil {
		return nil, err
	}
	if fa.Kind == VKList && fb.Kind == VKList {
		out := append(append([]*Value{}, fa.List...), fb.List...)
		return List(out...), nil
	}
	if fa.Kind == VKString || fb.Kind == VKString || fa.Kind == VKPath || fb.Kind == VKPath {
		return ev.concatCoercedStrings(fa, fb, pos)
	}
	return nil, evalErr(pos, "cannot concatenate values")
}

// mergeAttrBindMaps returns lhs // rhs attribute bindings without forcing values.
func mergeAttrBindMaps(lhs, rhs *Value) *Value {
	left := lhs
	if left == nil || left.Kind != VKAttrs {
		left = Attrs(false)
	}
	right := rhs
	if right == nil || right.Kind != VKAttrs {
		return left
	}
	out := Attrs(false)
	for sym, bind := range left.Attrs.Binds {
		out.Attrs.Binds[sym] = bind
	}
	for sym, bind := range right.Attrs.Binds {
		out.Attrs.Binds[sym] = bind
	}
	return out
}

func (ev *Evaluator) equalValues(a, b *Value) (bool, error) {
	pos := parser.Pos{}
	fa, err := ev.mustWHNF(a, pos)
	if err != nil {
		return false, err
	}
	fb, err := ev.mustWHNF(b, pos)
	if err != nil {
		return false, err
	}
	if fa == fb {
		return true, nil
	}
	if fa.Kind == VKNull || fb.Kind == VKNull {
		return fa.Kind == VKNull && fb.Kind == VKNull, nil
	}
	if fa.Kind != fb.Kind {
		return false, nil
	}
	switch fa.Kind {
	case VKBool:
		return fa.Bool == fb.Bool, nil
	case VKInt:
		return fa.Int == fb.Int, nil
	case VKFloat:
		return fa.Float == fb.Float || (math.IsNaN(fa.Float) && math.IsNaN(fb.Float)), nil
	case VKString:
		return fa.String == fb.String, nil
	case VKPath:
		return fa.Path == fb.Path, nil
	case VKList:
		if len(fa.List) != len(fb.List) {
			return false, nil
		}
		for i := range fa.List {
			eq, err := ev.equalValues(fa.List[i], fb.List[i])
			if err != nil || !eq {
				return eq, err
			}
		}
		return true, nil
	case VKAttrs:
		if fb.Kind != VKAttrs {
			return false, nil
		}
		if len(fa.Attrs.Binds) != len(fb.Attrs.Binds) {
			return false, nil
		}
		for sym, bindA := range fa.Attrs.Binds {
			bindB := fb.Attrs.Binds[sym]
			if bindB == nil {
				return false, nil
			}
			va, err := ev.attrBindingValue(bindA)
			if err != nil {
				return false, err
			}
			vb, err := ev.attrBindingValue(bindB)
			if err != nil {
				return false, err
			}
			eq, err := ev.equalValues(va, vb)
			if err != nil || !eq {
				return eq, err
			}
		}
		return true, nil
	default:
		return false, nil
	}
}

func (ev *Evaluator) describeThunk(t *Thunk) string {
	if t == nil {
		return "<nil>"
	}
	if t.errCtx != "" {
		return t.errCtx
	}
	if t.lazy != nil {
		return fmt.Sprintf("lazy@%s:%d", t.pos.File, t.pos.Line)
	}
	if t.expr != nil {
		switch e := t.expr.(type) {
		case *parser.Var:
			return fmt.Sprintf("var %q@%s:%d", e.Name, e.P.File, e.P.Line)
		case *parser.Select:
			return fmt.Sprintf("select@%s:%d", e.P.File, e.P.Line)
		case *parser.SelectOr:
			return fmt.Sprintf("selectOr@%s:%d", e.P.File, e.P.Line)
		case *parser.Call:
			return fmt.Sprintf("call@%s:%d", e.P.File, e.P.Line)
		default:
			return fmt.Sprintf("%T@%s:%d", e, t.pos.File, t.pos.Line)
		}
	}
	return fmt.Sprintf("thunk@%s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
}

func (ev *Evaluator) attrBindingValue(b *AttrBinding) (*Value, error) {
	if b == nil {
		return Null(), nil
	}
	if b.Thunk != nil {
		return ev.forceThunk(b.Thunk)
	}
	return b.Value, nil
}

// attrsBindingForce is an alias used by derivation and attr coercion call sites.
func (ev *Evaluator) attrsBindingForce(b *AttrBinding) (*Value, error) {
	return ev.attrBindingValue(b)
}
