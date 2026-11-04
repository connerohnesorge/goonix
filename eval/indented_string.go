// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"strings"
	"unicode"
)

// finishIndentedString applies Nix '' string unescaping and common-indent stripping.
func finishIndentedString(s string) string {
	s = unescapeIndentedString(s)
	return stripIndentedString(s)
}

func unescapeIndentedString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '\'' && s[i+1] == '\'' {
			if i+2 >= len(s) {
				b.WriteString("''")
				break
			}
			switch s[i+2] {
			case '\'':
				b.WriteByte('\'')
				i += 3
			case '$':
				if i+3 < len(s) && s[i+3] == '{' {
					b.WriteString("${")
					i += 4
				} else {
					b.WriteByte('$')
					i += 3
				}
			case '\\':
				b.WriteByte('\\')
				i += 3
			default:
				b.WriteString("''")
				i += 2
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func stripIndentedString(s string) string {
	if s == "" {
		return s
	}
	// Drop a single leading newline after the opening ''.
	if strings.HasPrefix(s, "\r\n") {
		s = s[2:]
	} else if strings.HasPrefix(s, "\n") {
		s = s[1:]
	}
	lines := strings.Split(s, "\n")
	min := -1
	for i, line := range lines {
		if i == len(lines)-1 && strings.HasSuffix(s, "\n") {
			// trailing empty from final newline before closing '' — keep as empty line
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := leadingIndentLen(line)
		if min < 0 || n < min {
			min = n
		}
	}
	if min <= 0 {
		return strings.Join(lines, "\n")
	}
	for i, line := range lines {
		if len(line) >= min && isSpacePrefix(line[:min]) {
			lines[i] = line[min:]
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// joinIndentedRawBodies concatenates '' string bodies before a single stripIndentedString.
func joinIndentedRawBodies(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out = strings.TrimRight(out, " \t")
		p = strings.TrimLeft(p, " \t")
		out = strings.TrimRight(out, "\n") + "\n" + strings.TrimLeft(p, "\n")
	}
	return out
}

func leadingIndentLen(line string) int {
	n := 0
	for _, r := range line {
		if r == ' ' || r == '\t' {
			n++
			continue
		}
		break
	}
	return n
}

func isSpacePrefix(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
