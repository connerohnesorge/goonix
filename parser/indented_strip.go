// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package parser

// indStringPart is one piece of a '' string before stripIndentation (Nix parser-state.hh).
type indStringPart struct {
	pos          Pos
	text         string
	hasIndent    bool
	interp       Expr
}

func stringLitExpr(pos Pos, s string) Expr {
	if s == "" {
		return &String{P: pos, Parts: []StringPart{{Pos: pos, Text: ""}}}
	}
	return &String{P: pos, Parts: []StringPart{{Pos: pos, Text: s}}}
}

// stripIndentation implements Nix ParserState::stripIndentation (2.24.9).
func stripIndentation(pos Pos, parts []indStringPart) Expr {
	if len(parts) == 0 {
		return stringLitExpr(pos, "")
	}

	atStartOfLine := true
	minIndent := 1_000_000
	curIndent := 0
	for _, p := range parts {
		if p.interp != nil || !p.hasIndent {
			if atStartOfLine {
				atStartOfLine = false
				if curIndent < minIndent {
					minIndent = curIndent
				}
			}
			continue
		}
		for j := 0; j < len(p.text); j++ {
			c := p.text[j]
			if atStartOfLine {
				if c == ' ' {
					curIndent++
				} else if c == '\n' {
					curIndent = 0
				} else {
					atStartOfLine = false
					if curIndent < minIndent {
						minIndent = curIndent
					}
				}
			} else if c == '\n' {
				atStartOfLine = true
				curIndent = 0
			}
		}
	}
	if minIndent == 1_000_000 {
		minIndent = 0
	}

	var out []Expr
	atStartOfLine = true
	curDropped := 0
	remaining := len(parts)

	flushString := func(partPos Pos, t string, last bool) {
		s2 := make([]byte, 0, len(t))
		for j := 0; j < len(t); j++ {
			c := t[j]
			if atStartOfLine {
				if c == ' ' {
					if curDropped >= minIndent {
						s2 = append(s2, c)
					}
					curDropped++
				} else if c == '\n' {
					curDropped = 0
					s2 = append(s2, c)
				} else {
					atStartOfLine = false
					curDropped = 0
					s2 = append(s2, c)
				}
			} else {
				s2 = append(s2, c)
				if c == '\n' {
					atStartOfLine = true
				}
			}
		}
		s := string(s2)
		if last {
			if p := lastIndexByte(s, '\n'); p >= 0 {
				tail := s[p+1:]
				allSpace := true
				for i := 0; i < len(tail); i++ {
					if tail[i] != ' ' {
						allSpace = false
						break
					}
				}
				if allSpace {
					s = s[:p+1]
				}
			}
		}
		if s != "" {
			out = append(out, stringLitExpr(partPos, s))
		}
	}

	for i, p := range parts {
		remaining = len(parts) - i
		if p.interp != nil {
			atStartOfLine = false
			curDropped = 0
			out = append(out, p.interp)
			continue
		}
		last := remaining == 1
		flushString(p.pos, p.text, last)
	}

	if len(out) == 0 {
		return stringLitExpr(pos, "")
	}
	if len(out) == 1 {
		if s, ok := out[0].(*String); ok && stringExprOnlyWhitespace(s) {
			return stringLitExpr(pos, "")
		}
	}
	if len(out) == 1 {
		return out[0]
	}
	return &ConcatStrings{P: pos, Parts: out}
}

func stringExprOnlyWhitespace(s *String) bool {
	for _, p := range s.Parts {
		if p.Interp != nil {
			return false
		}
		for i := 0; i < len(p.Text); i++ {
			c := p.Text[i]
			if c != ' ' && c != '\n' && c != '\t' && c != '\r' {
				return false
			}
		}
	}
	return true
}

func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
