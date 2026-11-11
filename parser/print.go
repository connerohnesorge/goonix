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

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Print formats an expression like nix-instantiate --parse (Nix Expr::show).
func Print(expr Expr) string {
	var b bytes.Buffer
	printExpr(&b, expr)
	return b.String()
}

func printExpr(b *bytes.Buffer, expr Expr) {
	switch e := expr.(type) {
	case *Integer:
		b.WriteString(e.Text)
	case *Float:
		b.WriteString(e.Text)
	case *Var:
		b.WriteString(e.Name)
	case *CurPos:
		b.WriteString("__curPos")
	case *String:
		if len(e.Parts) == 1 && e.Parts[0].Interp == nil {
			printLiteralString(b, escapeString(e.Parts[0].Text))
		} else {
			b.WriteString("(")
			first := true
			for _, part := range e.Parts {
				if !first {
					b.WriteString(" + ")
				}
				first = false
				if part.Interp != nil {
					printExpr(b, part.Interp)
				} else {
					printLiteralString(b, escapeString(part.Text))
				}
			}
			b.WriteString(")")
		}
	case *IndentedString:
		b.WriteString("''")
		for _, part := range e.Parts {
			if part.Interp != nil {
				fmt.Fprintf(b, "${")
				printExpr(b, part.Interp)
				b.WriteString("}")
			} else {
				b.WriteString(escapeIndented(part.Text))
			}
		}
		b.WriteString("''")
	case *Path:
		b.WriteString(e.Text)
	case *URI:
		b.WriteString(e.Text)
	case *List:
		b.WriteString("[ ")
		for _, elem := range e.Elems {
			b.WriteString("(")
			printExpr(b, elem)
			b.WriteString(") ")
		}
		b.WriteString("]")
	case *AttrSet:
		if e.Recursive {
			b.WriteString("rec ")
		}
		b.WriteString("{ ")
		printBindings(b, e)
		b.WriteString("}")
	case *Call:
		b.WriteString("(")
		printExpr(b, e.Fun)
		for _, arg := range e.Args {
			b.WriteByte(' ')
			printExpr(b, arg)
		}
		b.WriteString(")")
	case *Select:
		b.WriteString("(")
		printExpr(b, e.Expr)
		b.WriteString(").")
		printAttrPath(b, e.Path)
	case *SelectOr:
		b.WriteString("(")
		printExpr(b, e.Expr)
		b.WriteString(").")
		printAttrPath(b, e.Path)
		b.WriteString(" or (")
		printExpr(b, e.Default)
		b.WriteString(")")
	case *HasAttr:
		b.WriteString("((")
		printExpr(b, e.Expr)
		b.WriteString(") ? ")
		printAttrPath(b, e.Path)
		b.WriteString(")")
	case *UnaryOp:
		if e.Op == "!" {
			b.WriteString("(! ")
			printExpr(b, e.Expr)
			b.WriteString(")")
		} else {
			b.WriteString("(0 sub ")
			printExpr(b, e.Expr)
			b.WriteString(")")
		}
	case *BinaryOp:
		fmt.Fprintf(b, "(")
		printExpr(b, e.Left)
		fmt.Fprintf(b, " %s ", e.Op)
		printExpr(b, e.Right)
		b.WriteString(")")
	case *ConcatStrings:
		b.WriteString("(")
		for i, part := range e.Parts {
			if i > 0 {
				b.WriteString(" + ")
			}
			printExpr(b, part)
		}
		b.WriteString(")")
	case *If:
		b.WriteString("(if ")
		printExpr(b, e.Cond)
		b.WriteString(" then ")
		printExpr(b, e.Then)
		b.WriteString(" else ")
		printExpr(b, e.Else)
		b.WriteString(")")
	case *Assert:
		b.WriteString("assert ")
		printExpr(b, e.Cond)
		b.WriteString("; ")
		printExpr(b, e.Body)
	case *With:
		b.WriteString("(with ")
		printExpr(b, e.Attrs)
		b.WriteString("; ")
		printExpr(b, e.Body)
		b.WriteString(")")
	case *Let:
		b.WriteString("(let ")
		printBindings(b, e.Attrs)
		b.WriteString("in ")
		printExpr(b, e.Body)
		b.WriteString(")")
	case *Lambda:
		b.WriteString("(")
		if e.Formals != nil {
			printFormals(b, e.Formals)
			if e.Arg != "" && !e.ArgFirst {
				b.WriteString(" @ ")
			}
		}
		if e.Arg != "" {
			b.WriteString(e.Arg)
			if e.Formals != nil && e.ArgFirst {
				b.WriteString(" @ ")
			}
		}
		b.WriteString(": ")
		printExpr(b, e.Body)
		b.WriteString(")")
	case *Parens:
		printExpr(b, e.Expr)
	default:
		fmt.Fprintf(b, "<unknown %T>", expr)
	}
}

func printBindings(b *bytes.Buffer, set *AttrSet) {
	if set == nil {
		return
	}
	type bindSort struct {
		name string
		b    Binding
	}
	var plain []bindSort
	for _, bind := range set.Binds {
		plain = append(plain, bindSort{name: attrPathStaticName(bind.Path), b: bind})
	}
	sort.Slice(plain, func(i, j int) bool {
		return plain[i].name < plain[j].name
	})
	for _, inh := range set.Inherit {
		if inh.From == nil {
			b.WriteString("inherit")
			for _, n := range inh.Names {
				b.WriteByte(' ')
				printAttrName(b, n)
			}
			b.WriteString("; ")
		} else {
			b.WriteString("inherit (")
			printExpr(b, inh.From)
			b.WriteString(")")
			for _, n := range inh.Names {
				b.WriteByte(' ')
				printAttrName(b, n)
			}
			b.WriteString("; ")
		}
	}
	for _, item := range plain {
		printAttrPath(b, item.b.Path)
		b.WriteString(" = ")
		printExpr(b, item.b.Value)
		b.WriteString("; ")
	}
}

func attrPathStaticName(path []AttrName) string {
	var parts []string
	for _, p := range path {
		if p.Expr != nil {
			return "\xff"
		}
		parts = append(parts, p.Name)
	}
	return strings.Join(parts, ".")
}

func printFormals(b *bytes.Buffer, f *Formals) {
	names := make([]string, len(f.Params))
	for i, p := range f.Params {
		names[i] = p.Name
	}
	sort.Strings(names)
	b.WriteString("{ ")
	first := true
	for _, name := range names {
		for _, p := range f.Params {
			if p.Name != name {
				continue
			}
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(p.Name)
			if p.Default != nil {
				b.WriteString(" ? ")
				printExpr(b, p.Default)
			}
		}
	}
	if f.Ellipsis {
		if !first {
			b.WriteString(", ")
		}
		b.WriteString("...")
	}
	b.WriteString(" }")
}

func printAttrPath(b *bytes.Buffer, path []AttrName) {
	for i, seg := range path {
		if i > 0 {
			b.WriteByte('.')
		}
		printAttrName(b, seg)
	}
}

func printAttrName(b *bytes.Buffer, seg AttrName) {
	if seg.Expr != nil {
		b.WriteString("\"${")
		printExpr(b, seg.Expr)
		b.WriteString("}\"")
	} else {
		b.WriteString(seg.Name)
	}
}

func escapeString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "$", `\$`)
	return s
}

func escapeIndented(s string) string {
	s = strings.ReplaceAll(s, `''`, `'''`)
	s = strings.ReplaceAll(s, `$`, `''$`)
	s = strings.ReplaceAll(s, `\`, `''\`)
	return s
}

func printLiteralString(b *bytes.Buffer, content string) {
	b.WriteByte('"')
	b.WriteString(content)
	b.WriteByte('"')
}
