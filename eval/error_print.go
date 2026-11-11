// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

// errorPrint shows v the way Nix 2.24.9 prints a value inside an error
// message (its errorPrintOptions): nothing is forced, so an unevaluated
// value is «thunk», except literals, which Nix never wraps in a thunk.
// At most 10 list items and attributes are shown, nested 10 deep.
func (ev *Evaluator) errorPrint(v *Value) string {
	var b strings.Builder
	ev.errorPrintTo(&b, v, 0)
	return b.String()
}

const errorPrintMax = 10

func (ev *Evaluator) errorPrintTo(b *strings.Builder, v *Value, depth int) {
	v = ev.peekValue(v)
	if v == nil {
		b.WriteString("«thunk»")
		return
	}
	switch v.Kind {
	case VKNull:
		b.WriteString("null")
	case VKBool:
		b.WriteString(strconv.FormatBool(v.Bool))
	case VKInt:
		b.WriteString(strconv.FormatInt(v.Int, 10))
	case VKFloat:
		b.WriteString(strconv.FormatFloat(v.Float, 'g', 6, 64))
	case VKString:
		b.WriteString(`"` + errorStringEscaper.Replace(v.String) + `"`)
	case VKPath:
		b.WriteString(v.Path)
	case VKList:
		if len(v.List) == 0 {
			b.WriteString("[ ]")
			return
		}
		if depth >= errorPrintMax {
			b.WriteString("[ ... ]")
			return
		}
		b.WriteString("[ ")
		for i, el := range v.List {
			if i == errorPrintMax {
				fmt.Fprintf(b, "«%d items elided» ", len(v.List)-errorPrintMax)
				break
			}
			ev.errorPrintTo(b, el, depth+1)
			b.WriteByte(' ')
		}
		b.WriteString("]")
	case VKAttrs:
		if len(v.Attrs.Binds) == 0 {
			b.WriteString("{ }")
			return
		}
		if depth >= errorPrintMax {
			b.WriteString("{ ... }")
			return
		}
		names := make([]string, 0, len(v.Attrs.Binds))
		for sym := range v.Attrs.Binds {
			names = append(names, ev.Intern.Lookup(sym))
		}
		sort.Strings(names)
		b.WriteString("{ ")
		for i, name := range names {
			if i == errorPrintMax {
				fmt.Fprintf(b, "«%d attributes elided» ", len(names)-errorPrintMax)
				break
			}
			if parser.IsPlainIdent(name) {
				b.WriteString(name)
			} else {
				b.WriteString(`"` + errorStringEscaper.Replace(name) + `"`)
			}
			b.WriteString(" = ")
			bind := v.Attrs.Binds[ev.Intern.Intern(name)]
			val := bind.Value
			if bind.Thunk != nil {
				val = Deferred(bind.Thunk)
			}
			ev.errorPrintTo(b, val, depth+1)
			b.WriteString("; ")
		}
		b.WriteString("}")
	case VKLambda:
		if e := v.Lambda.Expr; e != nil {
			fmt.Fprintf(b, "«lambda @ %s:%d:%d»", e.P.File, e.P.Line, e.P.Col)
		} else {
			b.WriteString("«lambda»")
		}
	case VKBuiltin:
		b.WriteString("«primop " + v.Builtin.Name + "»")
	case VKPartialBuiltin:
		b.WriteString("«partially applied primop " + v.Partial.B.Name + "»")
	default:
		b.WriteString("«" + valueKindName(v.Kind) + "»")
	}
}

// peekValue returns v's value without evaluating anything new: a forced
// thunk's result, a literal's value, or a variable already bound to a value.
// It returns nil for anything else, which prints as «thunk».
func (ev *Evaluator) peekValue(v *Value) *Value {
	for v != nil && v.Kind == VKDeferred {
		t := v.Deferred
		if t.memo != nil {
			v = t.memo
			continue
		}
		if t.lazy != nil {
			return nil
		}
		switch e := t.expr.(type) {
		case *parser.Integer, *parser.Float, *parser.Path:
			lit, err := ev.evalExpr(e, t.env)
			if err != nil {
				return nil
			}
			v = lit
		case *parser.String:
			if len(e.Parts) != 1 || e.Parts[0].Interp != nil {
				return nil
			}
			return Str(e.Parts[0].Text, e.P)
		case *parser.Var:
			sym := ev.Intern.Intern(e.Name)
			if bind, ok := t.env.lookup(sym); ok {
				if bind.Thunk != nil {
					v = Deferred(bind.Thunk)
				} else {
					v = bind.Value
				}
				continue
			}
			switch e.Name {
			case "true":
				return Bool(true)
			case "false":
				return Bool(false)
			case "null":
				return Null()
			}
			return nil
		default:
			return nil
		}
	}
	return v
}

var errorStringEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "${", `\${`)

// coerceTypeError is Nix's error for a value that cannot become a string.
func (ev *Evaluator) coerceTypeError(pos parser.Pos, v *Value) error {
	switch v.Kind {
	case VKBuiltin:
		return evalErr(pos, "cannot coerce the built-in function '%s' to a string: %s", v.Builtin.Name, ev.errorPrint(v))
	case VKPartialBuiltin:
		return evalErr(pos, "cannot coerce the partially applied built-in function '%s' to a string: %s", v.Partial.B.Name, ev.errorPrint(v))
	}
	return evalErr(pos, "cannot coerce %s to a string: %s", nixShowType(v), ev.errorPrint(v))
}
