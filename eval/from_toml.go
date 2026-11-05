// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/pelletier/go-toml/v2/unstable"
)

func parseTOMLDocument(ev *Evaluator, src string) (*Value, error) {
	var p unstable.Parser
	p.Reset([]byte(src))
	doc := &tomlDoc{ev: ev, root: Attrs(false), cur: nil, closed: map[*Value]bool{}}
	doc.cur = doc.root
	timestamps := ev.xp != nil && ev.xp["parse-toml-timestamps"]
	for p.NextExpression() {
		n := p.Expression()
		switch n.Kind {
		case unstable.Comment:
			continue
		case unstable.Table:
			t, err := doc.define(tomlKey(n), false)
			if err != nil {
				return nil, err
			}
			doc.cur = t
		case unstable.ArrayTable:
			t, err := doc.define(tomlKey(n), true)
			if err != nil {
				return nil, err
			}
			doc.cur = t
		case unstable.KeyValue:
			val, err := doc.value(n.Value(), timestamps)
			if err != nil {
				return nil, err
			}
			if err := doc.insert(doc.cur, tomlKey(n), val); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected TOML node %s", n.Kind)
		}
	}
	if err := p.Error(); err != nil {
		return nil, err
	}
	return doc.root, nil
}

type tomlDoc struct {
	ev     *Evaluator
	root   *Value
	cur    *Value
	closed map[*Value]bool // tables closed by a header or an inline table
}

func tomlKey(n *unstable.Node) []string {
	var keys []string
	it := n.Key()
	for it.Next() {
		keys = append(keys, string(it.Node().Data))
	}
	return keys
}

func (d *tomlDoc) put(set *Value, key string, val *Value) {
	set.Attrs.Binds[d.ev.Intern.Intern(key)] = &AttrBinding{Value: val}
}

func (d *tomlDoc) get(set *Value, key string) (*Value, bool) {
	b, ok := set.Attrs.Binds[d.ev.Intern.Intern(key)]
	if !ok || b == nil || b.Value == nil {
		return nil, false
	}
	return b.Value, true
}

// define creates a [table] or [[array]] header, descending into the last
// element of an array-of-tables the way TOML 1.0 specifies.
func (d *tomlDoc) define(keys []string, array bool) (*Value, error) {
	if len(keys) == 0 {
		return nil, fmt.Errorf("empty TOML table name")
	}
	cur := d.root
	for i, k := range keys {
		last := i == len(keys)-1
		existing, ok := d.get(cur, k)
		if !last {
			if !ok {
				t := Attrs(false)
				d.put(cur, k, t)
				cur = t
				continue
			}
			if existing.Kind == VKList && len(existing.List) > 0 && existing.List[len(existing.List)-1].Kind == VKAttrs {
				cur = existing.List[len(existing.List)-1]
				continue
			}
			if existing.Kind == VKAttrs {
				cur = existing
				continue
			}
			return nil, fmt.Errorf("TOML key %q is not a table", k)
		}
		if array {
			var list *Value
			if ok && existing.Kind == VKList {
				list = existing
			} else if ok {
				return nil, fmt.Errorf("TOML key %q is not an array of tables", k)
			} else {
				list = List()
				d.put(cur, k, list)
			}
			t := Attrs(false)
			list.List = append(list.List, t)
			return t, nil
		}
		if ok && existing.Kind == VKAttrs {
			// A header may reopen a table that dotted keys created implicitly.
			// A second header, or a header over an inline table, is an error.
			if d.closed[existing] {
				return nil, fmt.Errorf("[error] toml::insert_value: table (%q) already exists.", k)
			}
			d.closed[existing] = true
			return existing, nil
		}
		if ok {
			return nil, fmt.Errorf("[error] toml::insert_value: value (%q) already exists.", strings.Join(keys, "."))
		}
		t := Attrs(false)
		d.closed[t] = true
		d.put(cur, k, t)
		return t, nil
	}
	return cur, nil
}

func (d *tomlDoc) insert(cur *Value, keys []string, val *Value) error {
	if len(keys) == 0 {
		return fmt.Errorf("empty TOML key")
	}
	for i, k := range keys {
		if i == len(keys)-1 {
			if _, ok := d.get(cur, k); ok {
				return fmt.Errorf("[error] toml::insert_value: value (%q) already exists.", k)
			}
			d.put(cur, k, val)
			return nil
		}
		existing, ok := d.get(cur, k)
		if !ok {
			t := Attrs(false)
			d.put(cur, k, t)
			cur = t
			continue
		}
		if existing.Kind != VKAttrs {
			return fmt.Errorf("TOML key %q is not a table", k)
		}
		cur = existing
	}
	return nil
}

func (d *tomlDoc) value(n *unstable.Node, timestamps bool) (*Value, error) {
	if n == nil {
		return nil, fmt.Errorf("missing TOML value")
	}
	switch n.Kind {
	case unstable.String:
		return Str(string(n.Data), parser.Pos{}), nil
	case unstable.Bool:
		return Bool(string(n.Data) == "true"), nil
	case unstable.Integer:
		i, err := parseTOMLInt(string(n.Data))
		if err != nil {
			return nil, err
		}
		return Int(i), nil
	case unstable.Float:
		f, err := strconv.ParseFloat(strings.ReplaceAll(string(n.Data), "_", ""), 64)
		if err != nil {
			return nil, err
		}
		// inf and nan are Nix floats. toJSON prints them as null.
		return &Value{Kind: VKFloat, Float: f}, nil
	case unstable.Array:
		var elems []*Value
		for c := n.Child(); c != nil; c = c.Next() {
			el, err := d.value(c, timestamps)
			if err != nil {
				return nil, err
			}
			elems = append(elems, el)
		}
		return List(elems...), nil
	case unstable.InlineTable:
		t := Attrs(false)
		for c := n.Child(); c != nil; c = c.Next() {
			if c.Kind != unstable.KeyValue {
				continue
			}
			val, err := d.value(c.Value(), timestamps)
			if err != nil {
				return nil, err
			}
			if err := d.insert(t, tomlKey(c), val); err != nil {
				return nil, err
			}
		}
		d.closed[t] = true
		return t, nil
	case unstable.LocalDate, unstable.LocalTime, unstable.LocalDateTime, unstable.DateTime:
		if !timestamps {
			return nil, fmt.Errorf("Dates and times are not supported")
		}
		stamp := Attrs(false)
		d.put(stamp, "_type", Str("timestamp", parser.Pos{}))
		d.put(stamp, "value", Str(nixTOMLTimestamp(string(n.Data)), parser.Pos{}))
		return stamp, nil
	default:
		return nil, fmt.Errorf("TOML value %s", n.Kind)
	}
}

func parseTOMLInt(s string) (int64, error) {
	s = strings.ReplaceAll(s, "_", "")
	sign := int64(1)
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	} else if strings.HasPrefix(s, "-") {
		sign = -1
		s = s[1:]
	}
	base := 10
	switch {
	case strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X"):
		base = 16
		s = s[2:]
	case strings.HasPrefix(s, "0o") || strings.HasPrefix(s, "0O"):
		base = 8
		s = s[2:]
	case strings.HasPrefix(s, "0b") || strings.HasPrefix(s, "0B"):
		base = 2
		s = s[2:]
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil {
		return 0, err
	}
	return sign * n, nil
}

// nixTOMLTimestamp matches toml11's printer: a space between a date and a
// time becomes 'T'. The rest of the literal is kept.
func nixTOMLTimestamp(raw string) string {
	raw = strings.TrimSpace(raw)
	date, rest, ok := strings.Cut(raw, " ")
	if !ok || len(date) != 10 || len(rest) < 8 {
		return raw
	}
	return date + "T" + rest
}
