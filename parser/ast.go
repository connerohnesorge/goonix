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

package parser

// Pos is a source position.
type Pos struct {
	File string
	Line int
	Col  int
}

// Expr is the root interface for Nix expression AST nodes.
type Expr interface {
	exprNode()
	Pos() Pos
}

// StringPart is either literal text or an interpolated expression.
type StringPart struct {
	Pos  Pos
	Text string   // set when Interp is nil
	Interp Expr   // set for ${...} parts
}

func (StringPart) exprNode() {}

// Integer literal.
type Integer struct {
	P    Pos
	Text string
}

func (*Integer) exprNode()   {}
func (e *Integer) Pos() Pos { return e.P }

// Float literal.
type Float struct {
	P    Pos
	Text string
}

func (*Float) exprNode()   {}
func (e *Float) Pos() Pos { return e.P }

// Var is a variable reference.
type Var struct {
	P    Pos
	Name string
}

func (*Var) exprNode()   {}
func (e *Var) Pos() Pos { return e.P }

// String is a double-quoted string, possibly with interpolations.
type String struct {
	P     Pos
	Parts []StringPart
}

func (*String) exprNode()   {}
func (e *String) Pos() Pos { return e.P }

// IndentedString is a ''...'' string.
type IndentedString struct {
	P     Pos
	Parts []StringPart
}

func (*IndentedString) exprNode()   {}
func (e *IndentedString) Pos() Pos { return e.P }

// Path literal.
type Path struct {
	P    Pos
	Text string
}

func (*Path) exprNode()   {}
func (e *Path) Pos() Pos { return e.P }

// URI literal.
type URI struct {
	P    Pos
	Text string
}

func (*URI) exprNode()   {}
func (e *URI) Pos() Pos { return e.P }

// List of expressions.
type List struct {
	P      Pos
	Elems  []Expr
}

func (*List) exprNode()   {}
func (e *List) Pos() Pos { return e.P }

// AttrName is one segment of an attribute path.
type AttrName struct {
	Name   string // set when Expr is nil
	Expr   Expr   // dynamic segment ${...}
}

// Binding is a single attribute binding.
type Binding struct {
	Pos   Pos
	Path  []AttrName
	Value Expr
	Kind  BindKind
}

type BindKind int

const (
	BindPlain BindKind = iota
	BindInherited
	BindInheritedFrom
)

// AttrSet is { ... } or rec { ... }.
type AttrSet struct {
	P         Pos
	Recursive bool
	Binds     []Binding
	Inherit   []Inherit
	// Members preserves source order of inherit and bind entries.
	Members []AttrSetMember
}

// AttrSetMember is one inherit or bind in an attribute set, in source order.
type AttrSetMember struct {
	Inherit *Inherit
	Binding *Binding
}

type Inherit struct {
	Pos    Pos
	From   Expr // nil for plain inherit
	Names  []AttrName
}

func (*AttrSet) exprNode()   {}
func (e *AttrSet) Pos() Pos { return e.P }

// Call is function application.
type Call struct {
	P    Pos
	Fun  Expr
	Args []Expr
}

func (*Call) exprNode()   {}
func (e *Call) Pos() Pos { return e.P }

// Select is e.attr or e.a.b.
type Select struct {
	P    Pos
	Expr Expr
	Path []AttrName
}

func (*Select) exprNode()   {}
func (e *Select) Pos() Pos { return e.P }

// SelectOr is e.attr or default.
type SelectOr struct {
	P       Pos
	Expr    Expr
	Path    []AttrName
	Default Expr
}

func (*SelectOr) exprNode()   {}
func (e *SelectOr) Pos() Pos { return e.P }

// HasAttr is e ? attrpath.
type HasAttr struct {
	P    Pos
	Expr Expr
	Path []AttrName
}

func (*HasAttr) exprNode()   {}
func (e *HasAttr) Pos() Pos { return e.P }

// UnaryOp is unary ! or -.
type UnaryOp struct {
	P    Pos
	Op   string
	Expr Expr
}

func (*UnaryOp) exprNode()   {}
func (e *UnaryOp) Pos() Pos { return e.P }

// BinaryOp is a binary operator expression.
type BinaryOp struct {
	P    Pos
	Op   string
	Left Expr
	Right Expr
}

func (*BinaryOp) exprNode()   {}
func (e *BinaryOp) Pos() Pos { return e.P }

// ConcatStrings is forced string concatenation with +.
type ConcatStrings struct {
	P     Pos
	Parts []Expr
}

func (*ConcatStrings) exprNode()   {}
func (e *ConcatStrings) Pos() Pos { return e.P }

// If expression.
type If struct {
	P    Pos
	Cond Expr
	Then Expr
	Else Expr
}

func (*If) exprNode()   {}
func (e *If) Pos() Pos { return e.P }

// Assert expression.
type Assert struct {
	P    Pos
	Cond Expr
	Body Expr
}

func (*Assert) exprNode()   {}
func (e *Assert) Pos() Pos { return e.P }

// With expression.
type With struct {
	P    Pos
	Attrs Expr
	Body Expr
}

func (*With) exprNode()   {}
func (e *With) Pos() Pos { return e.P }

// Let expression.
type Let struct {
	P    Pos
	Attrs *AttrSet
	Body Expr
}

func (*Let) exprNode()   {}
func (e *Let) Pos() Pos { return e.P }

// Formal is one formal parameter.
type Formal struct {
	P    Pos
	Name string
	Default Expr
}

// Formals is the parameter set of a lambda.
type Formals struct {
	P         Pos
	Params    []Formal
	Ellipsis  bool
}

// Lambda is a function.
type Lambda struct {
	P        Pos
	Arg      string // simple arg name, or empty if only formals
	Formals  *Formals
	Body     Expr
	ArgFirst bool // true when written arg @ formals :
}

func (*Lambda) exprNode()   {}
func (e *Lambda) Pos() Pos { return e.P }

// Parens wraps an expression (only used internally if needed).
type Parens struct {
	P    Pos
	Expr Expr
}

func (*Parens) exprNode()   {}
func (e *Parens) Pos() Pos { return e.P }

// CurPos is the __curPos pseudo-variable.
type CurPos struct {
	P Pos
}

func (*CurPos) exprNode()   {}
func (e *CurPos) Pos() Pos { return e.P }
