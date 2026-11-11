// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package parser

// decodeRegularStringText unescapes a "..." string chunk (Nix lexer.l unescapeStr).
func decodeRegularStringText(raw string) string {
	if raw == "" {
		return raw
	}
	var b []byte
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c == '\\' {
			if i+1 >= len(raw) {
				b = append(b, '\\')
				break
			}
			i++
			switch raw[i] {
			case 'n':
				b = append(b, '\n')
			case 'r':
				b = append(b, '\r')
			case 't':
				b = append(b, '\t')
			default:
				b = append(b, raw[i])
			}
			continue
		}
		if c == '\r' {
			b = append(b, '\n')
			if i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
			continue
		}
		b = append(b, c)
	}
	return string(b)
}
