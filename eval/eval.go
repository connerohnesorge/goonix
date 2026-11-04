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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
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
			v, err := ev.evalExpr(el, env)
			if err != nil {
				return nil, err
			}
			out[i] = v
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
		attrs, err := ev.evalExpr(e.Attrs, env)
		if err != nil {
			return nil, err
		}
		fattrs, err := ev.forceValue(attrs)
		if err != nil {
			return nil, err
		}
		if fattrs.Kind != VKAttrs {
			return nil, evalErr(e.P, "with expects an attribute set")
		}
		child := env.extend()
		child.With = fattrs
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
	if b, ok := ev.Builtins[v.Name]; ok {
		return &Value{Kind: VKBuiltin, Builtin: b}, nil
	}
	sym := ev.Intern.Intern(v.Name)
	if bind, ok := env.lookup(sym); ok {
		if bind.Thunk != nil {
			return Deferred(bind.Thunk), nil
		}
		return bind.Value, nil
	}
	return nil, evalErr(v.P, "undefined variable %q", v.Name)
}

func (ev *Evaluator) evalString(s *parser.String, env *Env) (*Value, error) {
	var b strings.Builder
	for _, part := range s.Parts {
		if part.Interp == nil {
			b.WriteString(part.Text)
			continue
		}
		v, err := ev.evalExpr(part.Interp, env)
		if err != nil {
			return nil, err
		}
		fv, err := ev.forceValue(v)
		if err != nil {
			return nil, err
		}
		b.WriteString(fv.DisplayString(ev, ev.Intern))
	}
	return Str(b.String(), s.P), nil
}

func (ev *Evaluator) evalIndentedString(s *parser.IndentedString, env *Env) (*Value, error) {
	var b strings.Builder
	for _, part := range s.Parts {
		if part.Interp == nil {
			b.WriteString(part.Text)
			continue
		}
		v, err := ev.evalExpr(part.Interp, env)
		if err != nil {
			return nil, err
		}
		fv, err := ev.forceValue(v)
		if err != nil {
			return nil, err
		}
		b.WriteString(fv.DisplayString(ev, ev.Intern))
	}
	return Str(b.String(), s.P), nil
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
	bindEnv := env.extend()

	if a.Recursive {
		for _, bind := range a.Binds {
			sym := ev.bindingSym(bind)
			if sym == 0 {
				continue
			}
			t := ev.newThunk(bind.Value, bindEnv, bind.Pos)
			bindEnv.bindThunk(sym, t)
			av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		}
	} else {
		for _, bind := range a.Binds {
			sym := ev.bindingSym(bind)
			if sym == 0 {
				continue
			}
			t := ev.newThunk(bind.Value, bindEnv, bind.Pos)
			av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
		}
	}

	for _, inh := range a.Inherit {
		if err := ev.bindAttrInherit(inh, bindEnv, av); err != nil {
			return nil, err
		}
	}

	return av, nil
}

func (ev *Evaluator) bindAttrInherit(inh parser.Inherit, env *Env, av *Value) error {
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
		t := ev.newThunk(expr, env, inh.Pos)
		av.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
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
			if bind, ok := env.lookup(sym); ok {
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
		for _, inh := range l.Attrs.Inherit {
			if err := ev.bindLetInherit(inh, child); err != nil {
				return nil, err
			}
		}
		for _, bind := range l.Attrs.Binds {
			sym := ev.bindingSym(bind)
			if sym == 0 {
				continue
			}
			child.bindThunk(sym, ev.newThunk(bind.Value, child, bind.Pos))
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
		env.bindThunk(sym, ev.newThunk(expr, env, inh.Pos))
	}
	return nil
}

func (ev *Evaluator) evalCall(c *parser.Call, env *Env) (*Value, error) {
	fun, err := ev.evalExpr(c.Fun, env)
	if err != nil {
		return nil, err
	}
	ff, err := ev.forceValue(fun)
	if err != nil {
		return nil, err
	}
	args := make([]*Value, len(c.Args))
	for i, a := range c.Args {
		args[i], err = ev.evalExpr(a, env)
		if err != nil {
			return nil, err
		}
	}
	switch ff.Kind {
	case VKLambda:
		result := ff
		for _, arg := range args {
			r, err := ev.applyLambda(result.Lambda, []*Value{arg}, c.P)
			if err != nil {
				return nil, err
			}
			fr, err := ev.forceValue(r)
			if err != nil {
				return nil, err
			}
			result = fr
		}
		return result, nil
	case VKBuiltin:
		return ff.Builtin.Fn(ev, c.P, args)
	default:
		return nil, evalErr(c.P, "attempt to call non-function")
	}
}

func (ev *Evaluator) applyLambda(l *LambdaValue, args []*Value, pos parser.Pos) (*Value, error) {
	if l == nil {
		return nil, evalErr(pos, "invalid lambda")
	}
	child := l.Env.extend()
	// Simple arg: x: body
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
	if l.SimpleArg != "" && len(args) == 1 {
		child.bind(ev.Intern.Intern(l.SimpleArg), args[0])
	}
	named := make(map[string]*Value)
	var rest []*Value
	if len(args) == 1 {
		fa, err := ev.forceValue(args[0])
		if err != nil {
			return nil, err
		}
		if fa.Kind == VKAttrs {
			for sym, bind := range fa.Attrs.Binds {
				name := ev.Intern.Lookup(sym)
				formalSym := ev.Intern.Intern(name)
				if bind.Thunk != nil {
					child.bindThunk(formalSym, bind.Thunk)
				} else {
					child.bind(formalSym, bind.Value)
				}
			}
		}
	} else {
		rest = args
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
			dv, err := ev.evalExpr(f.Default, child)
			if err != nil {
				return nil, err
			}
			child.bind(formalSym, dv)
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
	fb, err := ev.forceValue(base)
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
		if fb.Kind == VKAttrs {
			b, ok := fb.Attrs.Binds[sym]
			if !ok {
				if hasDefault && i == len(s.Path)-1 {
					return ev.evalExpr(defaultExpr, env)
				}
				return nil, evalErr(s.P, "attribute %q missing", name)
			}
			if b.Thunk != nil {
				fb, err = ev.forceThunk(b.Thunk)
			} else {
				fb = b.Value
			}
			if err != nil {
				return nil, err
			}
			continue
		}
		return nil, evalErr(s.P, "value is not an attribute set")
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
	fb, err := ev.forceValue(base)
	if err != nil {
		return nil, err
	}
	if fb.Kind != VKAttrs {
		return Bool(false), nil
	}
	for _, seg := range h.Path {
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
		if b.Thunk != nil {
			fb, err = ev.forceThunk(b.Thunk)
		} else {
			fb = b.Value
		}
		if err != nil {
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
	fv, err := ev.forceValue(v)
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
		return ev.concatValues(l, r)
	case "//":
		l, err := ev.evalExpr(b.Left, env)
		if err != nil {
			return nil, err
		}
		r, err := ev.evalExpr(b.Right, env)
		if err != nil {
			return nil, err
		}
		return ev.updateAttrSet(l, r, b.P)
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
	fl, err := ev.forceValue(l)
	if err != nil {
		return nil, err
	}
	fr, err := ev.forceValue(r)
	if err != nil {
		return nil, err
	}
	if b.Op == "+" && (fl.Kind == VKString || fr.Kind == VKString) {
		ls, _ := ev.forceString(fl)
		rs, _ := ev.forceString(fr)
		return Str(ls+rs, b.P), nil
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
	for _, fv := range vals {
		b.WriteString(fv.DisplayString(ev, ev.Intern))
	}
	return Str(b.String(), c.P), nil
}

func (ev *Evaluator) newThunk(expr parser.Expr, env *Env, pos parser.Pos) *Thunk {
	if pos.File == "" && expr != nil {
		pos = expr.Pos()
	}
	return &Thunk{eval: ev, expr: expr, env: env, pos: pos}
}

func (ev *Evaluator) forceThunk(t *Thunk) (*Value, error) {
	if t == nil {
		return Null(), nil
	}
	if t.memo != nil {
		return t.memo, nil
	}
	if t.locked {
		if t.memo != nil {
			return t.memo, nil
		}
		return nil, evalErr(t.pos, "infinite recursion encountered at %s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col)
	}
	t.locked = true
	v, err := t.eval.evalExpr(t.expr, t.env)
	if err != nil {
		t.locked = false
		return nil, err
	}
	fv, err := t.eval.forceValue(v)
	if err != nil {
		t.locked = false
		return nil, err
	}
	t.memo = fv
	t.locked = false
	return fv, nil
}

func (ev *Evaluator) forceValue(v *Value) (*Value, error) {
	if v == nil {
		return Null(), nil
	}
	if v.Kind == VKDeferred && v.Deferred != nil {
		return ev.forceThunk(v.Deferred)
	}
	return v, nil
}

func (ev *Evaluator) forceBool(e *Evaluator, v *Value, env *Env) (bool, error) {
	if v == nil {
		return false, nil
	}
	fv, err := ev.forceValue(v)
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
	fv, err := ev.forceValue(v)
	if err != nil {
		return "", err
	}
	if fv.Kind == VKBuiltin && fv.Builtin != nil && fv.Builtin.Nullary {
		fv, err = fv.Builtin.Fn(ev, parser.Pos{}, nil)
		if err != nil {
			return "", err
		}
	}
	if fv.Kind == VKString {
		return fv.String, nil
	}
	if fv.Kind == VKPath {
		return fv.Path, nil
	}
	return fv.DisplayString(ev, ev.Intern), nil
}

func (ev *Evaluator) concatValues(a, b *Value) (*Value, error) {
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
	if fa.Kind == VKString || fb.Kind == VKString {
		as, _ := ev.forceString(fa)
		bs, _ := ev.forceString(fb)
		return Str(as+bs, parser.Pos{}), nil
	}
	return nil, evalErr(parser.Pos{}, "cannot concatenate values")
}

func (ev *Evaluator) updateAttrSet(l, r *Value, pos parser.Pos) (*Value, error) {
	fl, err := ev.forceValue(l)
	if err != nil {
		return nil, err
	}
	fr, err := ev.forceValue(r)
	if err != nil {
		return nil, err
	}
	if fl.Kind != VKAttrs || fr.Kind != VKAttrs {
		return nil, evalErr(pos, "// expects attribute sets")
	}
	out := Attrs(false)
	for sym, bind := range fl.Attrs.Binds {
		out.Attrs.Binds[sym] = bind
	}
	for sym, bind := range fr.Attrs.Binds {
		out.Attrs.Binds[sym] = bind
	}
	return out, nil
}

func (ev *Evaluator) equalValues(a, b *Value) (bool, error) {
	fa, err := ev.forceValue(a)
	if err != nil {
		return false, err
	}
	fb, err := ev.forceValue(b)
	if err != nil {
		return false, err
	}
	if fa.Kind != fb.Kind {
		return false, nil
	}
	switch fa.Kind {
	case VKNull:
		return true, nil
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
	default:
		return false, nil
	}
}
