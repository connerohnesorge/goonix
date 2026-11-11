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
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
		// Nix shows a float with C++ stream defaults: 6 significant digits,
		// so 1.0 is 1 and 3.141592654 is 3.14159.
		if f, err := strconv.ParseFloat(e.Text, 64); err == nil {
			b.WriteString(strconv.FormatFloat(f, 'g', 6, 64))
		} else {
			b.WriteString(e.Text)
		}
	case *Var:
		b.WriteString(e.Name)
	case *CurPos:
		b.WriteString("__curPos")
	case *String:
		if len(e.Parts) == 0 {
			b.WriteString(`""`)
		} else if len(e.Parts) == 1 && e.Parts[0].Interp == nil {
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
		b.WriteString(absPathLiteral(e))
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
		// Nix's grammar appends to an existing call, even through
		// parentheses: (f a) b is one call with two arguments.
		fun, args := e.Fun, e.Args
		for {
			inner := fun
			for {
				p, ok := inner.(*Parens)
				if !ok {
					break
				}
				inner = p.Expr
			}
			c, ok := inner.(*Call)
			if !ok {
				break
			}
			fun, args = c.Fun, append(append([]Expr{}, c.Args...), args...)
		}
		b.WriteString("(")
		printExpr(b, fun)
		for _, arg := range args {
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
			b.WriteString("(__sub 0 ")
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
		// Nix prints formals first and then "@ name", whichever order the
		// source used.
		if e.Formals != nil {
			printFormals(b, e.Formals)
			if e.Arg != "" {
				b.WriteString(" @ ")
			}
		}
		b.WriteString(e.Arg)
		b.WriteString(": ")
		printExpr(b, e.Body)
		b.WriteString(")")
	case *Parens:
		printExpr(b, e.Expr)
	default:
		fmt.Fprintf(b, "<unknown %T>", expr)
	}
}

// printBindings is Nix's ExprAttrs::showBindings: all plain inherits as one
// sorted inherit, then each inherit (e) with its names sorted (skipped when
// it has none), then plain attributes sorted by name, then dynamic ones in
// source order.
func printBindings(b *bytes.Buffer, set *AttrSet) {
	if set == nil {
		return
	}
	var plainInherit []string
	for _, inh := range set.Inherit {
		if inh.From != nil {
			continue
		}
		for _, n := range inh.Names {
			plainInherit = append(plainInherit, n.Name)
		}
	}
	sort.Strings(plainInherit)
	if len(plainInherit) > 0 {
		b.WriteString("inherit")
		for _, n := range plainInherit {
			b.WriteByte(' ')
			printAttrName(b, AttrName{Name: n})
		}
		b.WriteString("; ")
	}
	for _, inh := range set.Inherit {
		if inh.From == nil || len(inh.Names) == 0 {
			continue
		}
		names := make([]string, 0, len(inh.Names))
		for _, n := range inh.Names {
			names = append(names, n.Name)
		}
		sort.Strings(names)
		b.WriteString("inherit (")
		printExpr(b, inh.From)
		b.WriteString(")")
		for _, n := range names {
			b.WriteByte(' ')
			printAttrName(b, AttrName{Name: n})
		}
		b.WriteString("; ")
	}
	type bindSort struct {
		name string
		b    Binding
	}
	var plain []bindSort
	for _, bind := range set.Binds {
		plain = append(plain, bindSort{name: attrPathStaticName(bind.Path), b: bind})
	}
	sort.SliceStable(plain, func(i, j int) bool {
		return plain[i].name < plain[j].name
	})
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
	} else if IsPlainIdent(seg.Name) {
		b.WriteString(seg.Name)
	} else {
		printLiteralString(b, escapeString(seg.Name))
	}
}

// IsPlainIdent reports whether Nix prints the attribute name without quotes:
// an identifier that is not a reserved keyword.
func IsPlainIdent(name string) bool {
	switch name {
	case "":
		return false
	case "or":
		return true
	case "if", "then", "else", "assert", "with", "let", "in", "rec", "inherit":
		return false
	}
	for i, c := range name {
		ok := c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
			i > 0 && (c >= '0' && c <= '9' || c == '\'' || c == '-')
		if !ok {
			return false
		}
	}
	return true
}

// escapeString escapes s the way Nix's printString does.
var stringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "${", `\${`)

func escapeString(s string) string {
	return stringEscaper.Replace(s)
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

// absPathLiteral is the path Nix's parser stores for a path literal: a
// relative path is made absolute against the file's directory (the current
// directory for expressions that are not from a file), and ~/ is $HOME.
func absPathLiteral(e *Path) string {
	text := e.Text
	switch {
	case strings.HasPrefix(text, "<"):
		return text
	case strings.HasPrefix(text, "/"):
		return filepath.Clean(text)
	case strings.HasPrefix(text, "~/"):
		return filepath.Join(os.Getenv("HOME"), text[2:])
	}
	dir := filepath.Dir(e.P.File)
	if e.P.File == "" || e.P.File == "(string)" {
		dir, _ = os.Getwd()
	}
	return filepath.Join(dir, text)
}
