// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package parser

// decodeIndStringText normalizes CR and decodes '' escapes (Nix IND_STRING lexer rules).
// hasIndentation is false for escape-produced fragments (''$ '', ''\n, etc.).
func decodeIndStringText(raw string, hasIndent bool) (text string, indent bool) {
	if !hasIndent {
		return normalizeCR(raw), false
	}
	return normalizeCR(raw), true
}

func normalizeCR(s string) string {
	if s == "" {
		return s
	}
	var b []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '\r' {
			b = append(b, '\n')
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			continue
		}
		b = append(b, s[i])
	}
	return string(b)
}

// decodeIndStringEscape decodes a single ''-escape sequence starting at src[i] == '\''.
// Returns decoded text and new index after the escape.
func decodeIndStringEscape(src []byte, i int) (string, int) {
	if i+1 >= len(src) || src[i] != '\'' || src[i+1] != '\'' {
		return "", i
	}
	if i+2 >= len(src) {
		return "''", i + 2
	}
	switch src[i+2] {
	case '\'':
		return "''", i + 3
	case '$':
		if i+3 < len(src) && src[i+3] == '{' {
			return "${", i + 4
		}
		return "$", i + 3
	case '\\':
		if i+3 >= len(src) {
			return `\`, i + 3
		}
		switch src[i+3] {
		case 'n':
			return "\n", i + 4
		case 'r':
			return "\r", i + 4
		case 't':
			return "\t", i + 4
		default:
			return string(src[i+3]), i + 4
		}
	default:
		return "''", i + 2
	}
}
