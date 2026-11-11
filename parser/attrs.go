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
	"fmt"
	"strings"
)

// staticAttrName turns ${"a"} and "a" into the static name a, as Nix's
// grammar does. Interpolated names stay dynamic.
func staticAttrName(n AttrName) AttrName {
	if n.Expr == nil {
		return n
	}
	e := n.Expr
	for {
		pe, ok := e.(*Parens)
		if !ok {
			break
		}
		e = pe.Expr
	}
	if s, ok := e.(*String); ok {
		if len(s.Parts) == 0 {
			return AttrName{Name: "", Pos: n.pos()}
		}
		if len(s.Parts) == 1 && s.Parts[0].Interp == nil {
			return AttrName{Name: s.Parts[0].Text, Pos: n.pos()}
		}
	}
	return n
}

func unparen(e Expr) Expr {
	for {
		pe, ok := e.(*Parens)
		if !ok {
			return e
		}
		e = pe.Expr
	}
}

const staticIndexAt = 16

// findStatic returns the plain binding or inherit for name in set. Small
// sets are scanned; a set past staticIndexAt names gets an index.
func findStatic(set *AttrSet, name string) (bind *Binding, inherited *Pos) {
	if set.static == nil && len(set.Binds)+len(set.Inherit) > staticIndexAt {
		set.static = make(map[string]staticRef, 2*len(set.Binds))
		for i, b := range set.Binds {
			if len(b.Path) == 1 && b.Path[0].Expr == nil {
				if _, dup := set.static[b.Path[0].Name]; !dup {
					set.static[b.Path[0].Name] = staticRef{bind: i}
				}
			}
		}
		for _, inh := range set.Inherit {
			for _, n := range inh.Names {
				if _, dup := set.static[n.Name]; n.Expr == nil && !dup {
					set.static[n.Name] = staticRef{bind: -1, pos: inh.Pos}
				}
			}
		}
	}
	if set.static != nil {
		ref, ok := set.static[name]
		switch {
		case !ok:
			return nil, nil
		case ref.bind < 0:
			p := ref.pos
			return nil, &p
		default:
			return &set.Binds[ref.bind], nil
		}
	}
	for i := range set.Binds {
		b := &set.Binds[i]
		if len(b.Path) == 1 && b.Path[0].Expr == nil && b.Path[0].Name == name {
			return b, nil
		}
	}
	for _, inh := range set.Inherit {
		for _, n := range inh.Names {
			if n.Expr == nil && n.Name == name {
				p := inh.Pos
				return nil, &p
			}
		}
	}
	return nil, nil
}

func appendBind(set *AttrSet, b Binding) {
	if set.static != nil && len(b.Path) == 1 && b.Path[0].Expr == nil {
		if _, dup := set.static[b.Path[0].Name]; !dup {
			set.static[b.Path[0].Name] = staticRef{bind: len(set.Binds)}
		}
	}
	set.Binds = append(set.Binds, b)
	bc := b
	set.Members = append(set.Members, AttrSetMember{Binding: &bc})
}

func dupAttr(path []AttrName, pos, prev Pos) error {
	var names []string
	for _, n := range path {
		if n.Expr != nil {
			names = append(names, `"${...}"`)
		} else {
			names = append(names, n.Name)
		}
	}
	return fmt.Errorf("%s:%d:%d: attribute '%s' already defined at %s:%d:%d",
		pos.File, pos.Line, pos.Col, strings.Join(names, "."), prev.File, prev.Line, prev.Col)
}

// addAttr is Nix's ParserState::addAttr. It desugars a.b.c = e into nested
// attribute sets, merges two attribute set literals bound to the same name,
// and rejects any other duplicate. Every binding it leaves has a one-element
// path.
func addAttr(set *AttrSet, path []AttrName, val Expr, pos Pos) error {
	for i := range path {
		path[i] = staticAttrName(path[i])
	}
	for _, seg := range path[:len(path)-1] {
		if seg.Expr != nil {
			nested := &AttrSet{P: pos}
			appendBind(set, Binding{Pos: pos, Path: []AttrName{seg}, Value: nested})
			set = nested
			continue
		}
		b, inh := findStatic(set, seg.Name)
		switch {
		case inh != nil:
			return dupAttr(path, pos, *inh)
		case b != nil:
			nested, ok := unparen(b.Value).(*AttrSet)
			if !ok {
				return dupAttr(path, pos, b.Pos)
			}
			set = nested
		default:
			nested := &AttrSet{P: pos}
			appendBind(set, Binding{Pos: pos, Path: []AttrName{seg}, Value: nested})
			set = nested
		}
	}
	last := path[len(path)-1]
	if last.Expr != nil {
		appendBind(set, Binding{Pos: pos, Path: []AttrName{last}, Value: val})
		return nil
	}
	b, inh := findStatic(set, last.Name)
	switch {
	case inh != nil:
		return dupAttr(path, pos, *inh)
	case b != nil:
		into, ok1 := unparen(b.Value).(*AttrSet)
		from, ok2 := unparen(val).(*AttrSet)
		if !ok1 || !ok2 {
			return dupAttr(path, pos, b.Pos)
		}
		return mergeAttrs(into, from)
	default:
		appendBind(set, Binding{Pos: pos, Path: []AttrName{last}, Value: val})
		return nil
	}
}

// mergeAttrs moves the members of from into into, failing on a name both
// define.
func mergeAttrs(into, from *AttrSet) error {
	for _, m := range from.Members {
		if m.Inherit != nil {
			for _, n := range m.Inherit.Names {
				if n.Expr != nil {
					continue
				}
				if b, inh := findStatic(into, n.Name); b != nil || inh != nil {
					prev := m.Inherit.Pos
					if b != nil {
						prev = b.Pos
					} else {
						prev = *inh
					}
					return dupAttr([]AttrName{n}, prev, m.Inherit.Pos)
				}
			}
			inh := *m.Inherit
			into.Inherit = append(into.Inherit, inh)
			indexInherit(into, inh)
			into.Members = append(into.Members, AttrSetMember{Inherit: &inh})
			continue
		}
		b := *m.Binding
		if b.Path[0].Expr == nil {
			if prev, inh := findStatic(into, b.Path[0].Name); prev != nil || inh != nil {
				pp := Pos{}
				if prev != nil {
					pp = prev.Pos
				} else {
					pp = *inh
				}
				return dupAttr(b.Path, pp, b.Pos)
			}
		}
		appendBind(into, b)
	}
	return nil
}

// addInherit records inh in set, failing on a name that is already defined.
func addInherit(set *AttrSet, inh Inherit) error {
	for i, n := range inh.Names {
		n = staticAttrName(n)
		inh.Names[i] = n
		if n.Expr != nil {
			continue
		}
		if b, prev := findStatic(set, n.Name); b != nil || prev != nil {
			pp := Pos{}
			if b != nil {
				pp = b.Pos
			} else {
				pp = *prev
			}
			return dupAttr([]AttrName{n}, inh.Pos, pp)
		}
		for _, m := range inh.Names[:i] {
			if m.Expr == nil && m.Name == n.Name {
				return dupAttr([]AttrName{n}, inh.Pos, inh.Pos)
			}
		}
	}
	set.Inherit = append(set.Inherit, inh)
	indexInherit(set, inh)
	set.Members = append(set.Members, AttrSetMember{Inherit: &inh})
	return nil
}

// indexInherit records inherited names once set has an index.
func indexInherit(set *AttrSet, inh Inherit) {
	if set.static == nil {
		return
	}
	for _, n := range inh.Names {
		if _, dup := set.static[n.Name]; n.Expr == nil && !dup {
			set.static[n.Name] = staticRef{bind: -1, pos: inh.Pos}
		}
	}
}
