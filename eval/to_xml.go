// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

func builtinToXML(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	var b strings.Builder
	var ctx []StringContextElem
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n<expr>\n")
	if err := writeXMLValue(ev, &b, &ctx, args[0], 1); err != nil {
		return nil, err
	}
	b.WriteString("</expr>")
	return StrWithContext(b.String(), ctx, pos), nil
}

func writeXMLValue(ev *Evaluator, b *strings.Builder, ctx *[]StringContextElem, v *Value, depth int) error {
	fv, err := ev.forceValue(v)
	if err != nil {
		return err
	}
	if fv == nil {
		fv = Null()
	}
	pad := strings.Repeat("  ", depth)
	switch fv.Kind {
	case VKInt:
		b.WriteString(pad)
		b.WriteString(`<int value="`)
		b.WriteString(strconv.FormatInt(fv.Int, 10))
		b.WriteString("\" />\n")
	case VKBool:
		b.WriteString(pad)
		if fv.Bool {
			b.WriteString("<bool value=\"true\" />\n")
		} else {
			b.WriteString("<bool value=\"false\" />\n")
		}
	case VKFloat:
		b.WriteString(pad)
		b.WriteString(`<float value="`)
		b.WriteString(nixXMLFloat(fv.Float))
		b.WriteString("\" />\n")
	case VKString:
		*ctx = append(*ctx, fv.Ctx...)
		b.WriteString(pad)
		b.WriteString(`<string value="`)
		b.WriteString(xmlEscape(fv.String))
		b.WriteString("\" />\n")
	case VKPath:
		b.WriteString(pad)
		b.WriteString(`<path value="`)
		b.WriteString(xmlEscape(fv.Path))
		b.WriteString("\" />\n")
	case VKNull:
		b.WriteString(pad)
		b.WriteString("<null />\n")
	case VKList:
		b.WriteString(pad)
		b.WriteString("<list>\n")
		for _, el := range fv.List {
			if err := writeXMLValue(ev, b, ctx, el, depth+1); err != nil {
				return err
			}
		}
		b.WriteString(pad)
		b.WriteString("</list>\n")
	case VKAttrs:
		if typ, err := getAttrString(ev, fv, "type"); err == nil && typ == "derivation" {
			return writeXMLDerivation(ev, b, ctx, fv, depth)
		}
		b.WriteString(pad)
		b.WriteString("<attrs>\n")
		if err := writeXMLAttrs(ev, b, ctx, fv, depth+1); err != nil {
			return err
		}
		b.WriteString(pad)
		b.WriteString("</attrs>\n")
	case VKLambda:
		b.WriteString(pad)
		b.WriteString("<function>\n")
		writeXMLFunction(b, fv.Lambda, depth+1)
		b.WriteString(pad)
		b.WriteString("</function>\n")
	default:
		b.WriteString(pad)
		b.WriteString("<unevaluated />\n")
	}
	return nil
}

func writeXMLDerivation(ev *Evaluator, b *strings.Builder, ctx *[]StringContextElem, fv *Value, depth int) error {
	pad := strings.Repeat("  ", depth)
	drvPath, _ := getAttrString(ev, fv, "drvPath")
	outPath, _ := getAttrString(ev, fv, "outPath")
	b.WriteString(pad)
	b.WriteString("<derivation")
	if drvPath != "" {
		b.WriteString(` drvPath="`)
		b.WriteString(xmlEscape(drvPath))
		b.WriteString(`"`)
	}
	if outPath != "" {
		b.WriteString(` outPath="`)
		b.WriteString(xmlEscape(outPath))
		b.WriteString(`"`)
	}
	b.WriteString(">\n")
	if err := writeXMLAttrs(ev, b, ctx, fv, depth+1); err != nil {
		return err
	}
	b.WriteString(pad)
	b.WriteString("</derivation>\n")
	return nil
}

func writeXMLAttrs(ev *Evaluator, b *strings.Builder, ctx *[]StringContextElem, set *Value, depth int) error {
	pad := strings.Repeat("  ", depth)
	for _, name := range attrNames(ev, set) {
		sym := ev.Intern.Intern(name)
		bind := set.Attrs.Binds[sym]
		var v *Value
		if bind != nil && bind.Thunk != nil {
			var err error
			v, err = ev.forceThunk(bind.Thunk)
			if err != nil {
				return err
			}
		} else if bind != nil {
			v = bind.Value
		}
		b.WriteString(pad)
		b.WriteString(`<attr name="`)
		b.WriteString(xmlEscape(name))
		b.WriteString("\">\n")
		if err := writeXMLValue(ev, b, ctx, v, depth+1); err != nil {
			return err
		}
		b.WriteString(pad)
		b.WriteString("</attr>\n")
	}
	return nil
}

func writeXMLFunction(b *strings.Builder, l *LambdaValue, depth int) {
	pad := strings.Repeat("  ", depth)
	if l == nil || (l.Formals == nil && l.SimpleArg == "") {
		b.WriteString(pad)
		b.WriteString("<unevaluated />\n")
		return
	}
	if l.Formals == nil {
		b.WriteString(pad)
		b.WriteString(`<varpat name="`)
		b.WriteString(xmlEscape(l.SimpleArg))
		b.WriteString("\" />\n")
		return
	}
	b.WriteString(pad)
	b.WriteString("<attrspat")
	if l.SimpleArg != "" {
		b.WriteString(` name="`)
		b.WriteString(xmlEscape(l.SimpleArg))
		b.WriteString(`"`)
	}
	if l.Formals.Ellipsis {
		b.WriteString(` ellipsis="1"`)
	}
	b.WriteString(">\n")
	names := make([]string, 0, len(l.Formals.Params))
	for _, p := range l.Formals.Params {
		names = append(names, p.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		b.WriteString(pad)
		b.WriteString("  ")
		b.WriteString(`<attr name="`)
		b.WriteString(xmlEscape(name))
		b.WriteString("\" />\n")
	}
	b.WriteString(pad)
	b.WriteString("</attrspat>\n")
}

func attrNames(ev *Evaluator, set *Value) []string {
	if set == nil || set.Attrs == nil {
		return nil
	}
	keys := make([]string, 0, len(set.Attrs.Binds))
	for sym := range set.Attrs.Binds {
		keys = append(keys, ev.Intern.Lookup(sym))
	}
	sort.Strings(keys)
	return keys
}

func nixXMLFloat(f float64) string {
	if !math.IsInf(f, 0) && !math.IsNaN(f) && f == math.Trunc(f) && f >= math.MinInt64 && f <= math.MaxInt64 {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
