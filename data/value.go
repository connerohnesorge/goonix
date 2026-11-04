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

package data

// Kind classifies runtime values in the Nix-like evaluator.
type Kind uint8

const (
	KindNull Kind = iota + 1
	KindBool
	KindInt
	KindFloat
	KindString
	KindPath
	KindList
	KindAttrSet
	KindLambda
	KindPrimOp
	KindThunk
)

// Pos records source context for error messages.
type Pos struct {
	File   string
	Line   int
	Column int
}

// Value is a tagged union implemented with plain Go pointers (gonix-style).
type Value struct {
	Kind Kind

	Bool   bool
	Int    int64
	Float  float64
	String string
	Pos    Pos
	Path   string

	List   []*Value
	Attrs  map[Symbol]*Value
	Lambda *Lambda
	PrimOp PrimOp
	Thunk  *Thunk
}

// Lambda is a user function value (body evaluation deferred).
type Lambda struct {
	Param Symbol
	Body  *Value
}

// PrimOp identifies a builtin operator implemented in Go.
type PrimOp int

const (
	PrimAdd PrimOp = iota + 1
	PrimSub
	PrimMul
	PrimDiv
)

// Thunk wraps deferred computation.
type Thunk struct {
	Expr *Value
	Env  map[Symbol]*Value
}

// NullV returns the null value.
func NullV() *Value {
	return &Value{Kind: KindNull}
}

func BoolV(b bool) *Value {
	return &Value{Kind: KindBool, Bool: b}
}

func IntV(i int64) *Value {
	return &Value{Kind: KindInt, Int: i}
}

func FloatV(f float64) *Value {
	return &Value{Kind: KindFloat, Float: f}
}

func StringV(s string, pos Pos) *Value {
	return &Value{Kind: KindString, String: s, Pos: pos}
}

func PathV(p string) *Value {
	return &Value{Kind: KindPath, Path: p}
}

func ListV(elems ...*Value) *Value {
	return &Value{Kind: KindList, List: elems}
}

func AttrSetV(attrs map[Symbol]*Value) *Value {
	return &Value{Kind: KindAttrSet, Attrs: attrs}
}
