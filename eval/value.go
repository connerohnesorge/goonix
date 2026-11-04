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
	"strconv"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
)

// ValueKind classifies runtime values during evaluation.
type ValueKind int

const (
	VKNull ValueKind = iota + 1
	VKBool
	VKInt
	VKFloat
	VKString
	VKPath
	VKList
	VKAttrs
	VKLambda
	VKBuiltin
	VKPartialBuiltin
	VKDerivation
	VKDeferred // unevaluated let binding / lazy reference
)

// Value is a runtime Nix value.
type Value struct {
	Kind ValueKind

	Bool   bool
	Int    int64
	Float  float64
	String string
	Path   string
	Pos    parser.Pos

	List []*Value
	Attrs *AttrSet

	Lambda *LambdaValue
	Builtin *Builtin
	Partial *PartialBuiltin

	// Derivation metadata (type = "derivation").
	Derivation *DerivationValue

	// Deferred is a lazy let binding referenced before full evaluation.
	Deferred *Thunk
}

// AttrSet holds attribute bindings, optionally lazy/recursive.
type AttrSet struct {
	Recursive bool
	Binds     map[data.Symbol]*AttrBinding
}

// AttrBinding is one attribute, optionally deferred.
type AttrBinding struct {
	Value *Value
	Thunk *Thunk
}

// LambdaValue is a user-defined function closure.
type LambdaValue struct {
	SimpleArg string
	Formals   *parser.Formals
	ArgFirst  bool
	Body      parser.Expr
	Env       *Env
}

// DerivationValue holds evaluated derivation fields.
type DerivationValue struct {
	Name              string
	System            string
	Builder           string
	Args              []string
	Env               map[string]string
	Outputs           map[string]struct{}
	InputDerivations  map[string][]string
	InputSources      []string
	DrvPath           string
	OutPaths          map[string]string
}

// Thunk defers evaluation with memoisation and blackhole detection.
type Thunk struct {
	eval *Evaluator
	expr parser.Expr
	env  *Env
	pos  parser.Pos

	memo   *Value
	locked bool // blackhole marker while forcing
}

func (v *Value) force(e *Evaluator) (*Value, error) {
	if v == nil {
		return Null(), nil
	}
	if v.Kind != VKAttrs && v.Attrs != nil {
		// already forced attrs
	}
	return v, nil
}

func (v *Value) DisplayString(e *Evaluator, intern *data.Interner) string {
	switch v.Kind {
	case VKNull:
		return "null"
	case VKBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case VKInt:
		return strconv.FormatInt(v.Int, 10)
	case VKFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case VKString:
		return v.String
	case VKPath:
		return v.Path
	case VKList:
		var parts []string
		for _, el := range v.List {
			f, err := e.forceValue(el)
			if err != nil {
				return "<list>"
			}
			parts = append(parts, f.DisplayString(e, intern))
		}
		return "[ " + joinStrings(parts, " ") + " ]"
	case VKAttrs:
		return "{ ... }"
	case VKLambda:
		return "<lambda>"
	case VKBuiltin:
		return "<builtin>"
	case VKDeferred:
		return "<deferred>"
	case VKDerivation:
		if v.Derivation != nil && v.Derivation.OutPaths != nil {
			if p, ok := v.Derivation.OutPaths["out"]; ok {
				return p
			}
		}
		return "<derivation>"
	default:
		return fmt.Sprintf("<%d>", v.Kind)
	}
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}

// Null returns the null value.
func Null() *Value {
	return &Value{Kind: VKNull}
}

func Bool(b bool) *Value {
	return &Value{Kind: VKBool, Bool: b}
}

func Int(i int64) *Value {
	return &Value{Kind: VKInt, Int: i}
}

func Float(f float64) *Value {
	return &Value{Kind: VKFloat, Float: f}
}

func Str(s string, pos parser.Pos) *Value {
	return &Value{Kind: VKString, String: s, Pos: pos}
}

func Path(p string) *Value {
	return &Value{Kind: VKPath, Path: p}
}

func List(elems ...*Value) *Value {
	return &Value{Kind: VKList, List: elems}
}

// Deferred wraps a lazy binding without forcing it.
func Deferred(t *Thunk) *Value {
	return &Value{Kind: VKDeferred, Deferred: t}
}

func Attrs(recursive bool) *Value {
	return &Value{
		Kind: VKAttrs,
		Attrs: &AttrSet{
			Recursive: recursive,
			Binds:     make(map[data.Symbol]*AttrBinding),
		},
	}
}

func posToData(p parser.Pos) data.Pos {
	return data.Pos{File: p.File, Line: p.Line, Column: p.Col}
}

// ToData converts a forced eval Value into data.Value (shallow for containers).
func ToData(v *Value, intern *data.Interner) *data.Value {
	if v == nil {
		return data.NullV()
	}
	switch v.Kind {
	case VKNull:
		return data.NullV()
	case VKBool:
		return data.BoolV(v.Bool)
	case VKInt:
		return data.IntV(v.Int)
	case VKFloat:
		return data.FloatV(v.Float)
	case VKString:
		return data.StringV(v.String, posToData(v.Pos))
	case VKPath:
		return data.PathV(v.Path)
	case VKList:
		elems := make([]*data.Value, len(v.List))
		for i, el := range v.List {
			elems[i] = ToData(el, intern)
		}
		return data.ListV(elems...)
	case VKAttrs:
		m := make(map[data.Symbol]*data.Value, len(v.Attrs.Binds))
		for sym, bind := range v.Attrs.Binds {
			if bind.Value != nil {
				m[sym] = ToData(bind.Value, intern)
			} else {
				m[sym] = data.NullV()
			}
		}
		return data.AttrSetV(m)
	case VKLambda:
		if v.Lambda != nil && v.Lambda.SimpleArg != "" {
			return &data.Value{
				Kind: data.KindLambda,
				Lambda: &data.Lambda{
					Param: intern.Intern(v.Lambda.SimpleArg),
					Body:  data.NullV(),
				},
			}
		}
		return &data.Value{Kind: data.KindLambda}
	case VKBuiltin:
		return &data.Value{Kind: data.KindPrimOp}
	default:
		return data.NullV()
	}
}

// FromData lifts simple data values into eval values (no lambdas/thunks).
func FromData(d *data.Value) *Value {
	if d == nil {
		return Null()
	}
	switch d.Kind {
	case data.KindNull:
		return Null()
	case data.KindBool:
		return Bool(d.Bool)
	case data.KindInt:
		return Int(d.Int)
	case data.KindFloat:
		return Float(d.Float)
	case data.KindString:
		return Str(d.String, parser.Pos{File: d.Pos.File, Line: d.Pos.Line, Col: d.Pos.Column})
	case data.KindPath:
		return Path(d.Path)
	case data.KindList:
		elems := make([]*Value, len(d.List))
		for i, el := range d.List {
			elems[i] = FromData(el)
		}
		return List(elems...)
	case data.KindAttrSet:
		av := Attrs(false)
		for sym, val := range d.Attrs {
			av.Attrs.Binds[sym] = &AttrBinding{Value: FromData(val)}
		}
		return av
	default:
		return Null()
	}
}
