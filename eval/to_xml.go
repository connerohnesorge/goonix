// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

func builtinToXML(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "toXML expects attribute set")
	}
	var b strings.Builder
	b.WriteString("<?xml version='1.0' encoding='utf-8'?>\n<expr>\n  <attrs>\n")
	keys := attrNames(ev, set)
	for _, name := range keys {
		sym := ev.Intern.Intern(name)
		bind := set.Attrs.Binds[sym]
		var v *Value
		if bind.Thunk != nil {
			v, err = ev.forceThunk(bind.Thunk)
		} else {
			v = bind.Value
		}
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "    <attr name=%q>\n", name)
		if err := writeXMLValueIndent(ev, &b, v, "      "); err != nil {
			return nil, err
		}
		b.WriteString("    </attr>\n")
	}
	b.WriteString("  </attrs>\n</expr>")
	return Str(b.String(), pos), nil
}

func attrNames(ev *Evaluator, set *Value) []string {
	keys := make([]string, 0, len(set.Attrs.Binds))
	for sym := range set.Attrs.Binds {
		keys = append(keys, ev.Intern.Lookup(sym))
	}
	sort.Strings(keys)
	return keys
}

func writeXMLValueIndent(ev *Evaluator, b *strings.Builder, v *Value, indent string) error {
	switch v.Kind {
	case VKInt:
		fmt.Fprintf(b, "%s<int value=\"%d\" />\n", indent, v.Int)
	case VKBool:
		fmt.Fprintf(b, "%s<bool value=\"%t\" />\n", indent, v.Bool)
	case VKString:
		fmt.Fprintf(b, "%s<string value=%q />\n", indent, v.String)
	case VKNull:
		fmt.Fprintf(b, "%s<null />\n", indent)
	case VKList:
		fmt.Fprintf(b, "%s<list>\n", indent)
		for _, el := range v.List {
			if err := writeXMLValueIndent(ev, b, el, indent+"  "); err != nil {
				return err
			}
		}
		fmt.Fprintf(b, "%s</list>\n", indent)
	case VKAttrs:
		fmt.Fprintf(b, "%s<attrs>\n", indent)
		for _, name := range attrNames(ev, v) {
			fmt.Fprintf(b, "%s  <attr name=%q>\n", indent, name)
			fmt.Fprintf(b, "%s  </attr>\n", indent)
		}
		fmt.Fprintf(b, "%s</attrs>\n", indent)
	default:
		return fmt.Errorf("toXML: unsupported value kind %v", v.Kind)
	}
	return nil
}
