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
	"unicode/utf8"
)

const maxLexerStack = 64

type lexResult struct {
	file     string
	src      []byte
	tokens   []Token
	comments []Token
	lines    []int // byte offsets of line starts
}

func newLexResult(path string, src []byte) *lexResult {
	if path == "" {
		path = "(string)"
	}
	lines := []int{0}
	for i, b := range src {
		if b == '\n' {
			lines = append(lines, i+1)
		}
	}
	return &lexResult{file: path, src: src, lines: lines}
}

func (r *lexResult) pos(offset int) Pos {
	line := 1
	col := 1
	for i := len(r.lines) - 1; i >= 0; i-- {
		if r.lines[i] <= offset {
			line = i + 1
			col = offset - r.lines[i] + 1
			break
		}
	}
	return Pos{File: r.file, Line: line, Col: col}
}

func (r *lexResult) slice(start, end int) string {
	return string(r.src[start:end])
}

func (r *lexResult) errorAt(offset int, format string, args ...interface{}) error {
	p := r.pos(offset)
	return fmt.Errorf("%s:%d:%d: %s", p.File, p.Line, p.Col, fmt.Sprintf(format, args...))
}

func lex(src []byte, path string) (*lexResult, error) {
	r := newLexResult(path, src)
	var backrefs [][2]int // token index of opener, expected close type

	emit := func(typ TokenType, start, end int) {
		r.tokens = append(r.tokens, Token{
			Type:  typ,
			Pos:   r.pos(start),
			Start: start,
			End:   end,
		})
	}

	emitComment := func(start, end int) {
		r.comments = append(r.comments, Token{
			Type:  tokenComment,
			Pos:   r.pos(start),
			Start: start,
			End:   end,
		})
	}

	push := func(openTyp TokenType, closeTyp TokenType, start, end int) {
		emit(openTyp, start, end)
		backrefs = append(backrefs, [2]int{len(r.tokens) - 1, int(closeTyp)})
	}

	pop := func(closeTyp TokenType, start, end int) error {
		r.tokens = append(r.tokens, Token{
			Type:  closeTyp,
			Pos:   r.pos(start),
			Start: start,
			End:   end,
		})
		if len(backrefs) == 0 {
			return r.errorAt(start, "unexpected %q", r.slice(start, end))
		}
		br := backrefs[len(backrefs)-1]
		backrefs = backrefs[:len(backrefs)-1]
		opener := &r.tokens[br[0]]
		if TokenType(br[1]) != closeTyp {
			return r.errorAt(start, "does not close %s", opener.Type)
		}
		r.tokens[len(r.tokens)-1].Prev = br[0]
		return nil
	}

	reclassifyArg := func(start, end int) error {
		emit(tokenAt, start, end) // will fix type below for :
		last := &r.tokens[len(r.tokens)-1]
		if len(r.tokens) < 2 {
			return r.errorAt(start, "%q does not follow anything", r.slice(start, end))
		}
		ch := src[start]
		if ch == '@' {
			last.Type = tokenAt
		} else {
			last.Type = tokenColon
		}
		prev := &r.tokens[len(r.tokens)-2]
		switch prev.Type {
		case tokenID:
			prev.Type = tokenArgID
		case tokenRBrace:
			if prev.Prev < 0 {
				return r.errorAt(start, "%q does not follow a function argument", r.slice(start, end))
			}
			opener := &r.tokens[prev.Prev]
			if opener.Type != tokenLBrace && opener.Type != tokenArgBracket {
				return r.errorAt(start, "%q does not follow a function argument", r.slice(start, end))
			}
			if opener.Type == tokenLBrace {
				opener.Type = tokenArgBracket
			}
		default:
			return r.errorAt(start, "%q does not follow a function argument", r.slice(start, end))
		}
		return nil
	}

	i := 0
	n := len(src)

	var lexExpr func() error
	var lexDQuoteString func() error
	var lexIndentedString func() error

	lexDQuoteString = func() error {
		for i < n {
			start := i
			if src[i] == '"' {
				if err := pop(tokenDQuote, start, start+1); err != nil {
					return err
				}
				i++
				return nil
			}
			if i+1 < n && src[i] == '$' && src[i+1] == '{' {
				push(tokenInterpOpen, tokenInterpClose, i, i+2)
				i += 2
				if err := lexExpr(); err != nil {
					return err
				}
				continue
			}
			for i < n {
				if src[i] == '"' {
					break
				}
				if src[i] == '$' && i+1 < n && src[i+1] == '{' {
					break
				}
				if src[i] == '\\' && i+1 < n {
					i += 2
					continue
				}
				if src[i] == '\n' {
					i++
					continue
				}
				i++
			}
			if i > start {
				emit(tokenText, start, i)
			} else if i < n && src[i] == '$' && i+1 < n && src[i+1] != '{' {
				emit(tokenText, i, i+1)
				i++
			}
		}
		return r.errorAt(n, "unterminated string")
	}

	lexIndentedString = func() error {
		for i < n {
			start := i
			if i+1 < n && src[i] == '\'' && src[i+1] == '\'' {
				if err := pop(tokenIStringClose, start, start+2); err != nil {
					return err
				}
				i += 2
				return nil
			}
			if i+1 < n && src[i] == '$' && src[i+1] == '{' {
				push(tokenInterpOpen, tokenInterpClose, i, i+2)
				i += 2
				if err := lexExpr(); err != nil {
					return err
				}
				continue
			}
			for i < n {
				if i+1 < n && src[i] == '\'' && src[i+1] == '\'' {
					break
				}
				if i+1 < n && src[i] == '$' && src[i+1] == '{' {
					break
				}
				if i+2 < n && src[i] == '\'' && src[i+1] == '\'' && src[i+2] == '\\' {
					i += 3
					continue
				}
				if src[i] == '\'' && i+1 < n && src[i+1] != '\'' {
					i++
					continue
				}
				if src[i] == '$' {
					i++
					continue
				}
				i++
			}
			if i > start {
				emit(tokenText, start, i)
			}
		}
		return r.errorAt(n, "unterminated indented string")
	}

	lexExpr = func() error {
		for i < n {
			start := i

			if c := src[i]; c == ' ' || c == '\t' || c == '\r' {
				i++
				continue
			}
			if src[i] == '\n' {
				i++
				continue
			}

			// comments
			if src[i] == '#' {
				for i < n && src[i] != '\n' {
					i++
				}
				emitComment(start, i)
				continue
			}
			if i+1 < n && src[i] == '/' && src[i+1] == '*' {
				i += 2
				for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
					if src[i] == '\n' {
						// line tracking already in lines slice from init - ok for positions
					}
					i++
				}
				if i+1 >= n {
					return r.errorAt(start, "unterminated /* comment")
				}
				i += 2
				emitComment(start, i)
				continue
			}

			// two-char operators
			if i+1 < n {
				switch string(src[i : i+2]) {
				case "->":
					emit(tokenImpl, start, start+2)
					i += 2
					continue
				case "||":
					emit(tokenOr, start, start+2)
					i += 2
					continue
				case "&&":
					emit(tokenAnd, start, start+2)
					i += 2
					continue
				case "==":
					emit(tokenEqEq, start, start+2)
					i += 2
					continue
				case "!=":
					emit(tokenNotEq, start, start+2)
					i += 2
					continue
				case "<=":
					emit(tokenLeq, start, start+2)
					i += 2
					continue
				case ">=":
					emit(tokenGeq, start, start+2)
					i += 2
					continue
				case "//":
					emit(tokenUpdate, start, start+2)
					i += 2
					continue
				case "++":
					emit(tokenConcat, start, start+2)
					i += 2
					continue
				case "${":
					if len(backrefs) >= maxLexerStack {
						return r.errorAt(start, "exceeds recursion limit")
					}
					push(tokenInterpOpen, tokenInterpClose, start, start+2)
					i += 2
					if err := lexExpr(); err != nil {
						return err
					}
					continue
				case "''":
					push(tokenIStringOpen, tokenIStringClose, start, start+2)
					i += 2
					if err := lexIndentedString(); err != nil {
						return err
					}
					continue
				}
			}

			// ellipsis
			if i+2 < n && src[i] == '.' && src[i+1] == '.' && src[i+2] == '.' {
				emit(tokenEllipsis, start, start+3)
				i += 3
				continue
			}

			// single-char punct
			switch src[i] {
			case '@', ':':
				if err := reclassifyArg(start, start+1); err != nil {
					return err
				}
				i++
				continue
			case ',':
				emit(tokenComma, start, start+1)
				i++
				continue
			case ';':
				emit(tokenSemi, start, start+1)
				i++
				continue
			case '.':
				emit(tokenDot, start, start+1)
				i++
				continue
			case '(':
				if len(backrefs) >= maxLexerStack {
					return r.errorAt(start, "exceeds recursion limit")
				}
				push(tokenLParen, tokenRParen, start, start+1)
				i++
				if err := lexExpr(); err != nil {
					return err
				}
				continue
			case '[':
				if len(backrefs) >= maxLexerStack {
					return r.errorAt(start, "exceeds recursion limit")
				}
				push(tokenLBracket, tokenRBracket, start, start+1)
				i++
				if err := lexExpr(); err != nil {
					return err
				}
				continue
			case '{':
				if len(backrefs) >= maxLexerStack {
					return r.errorAt(start, "exceeds recursion limit")
				}
				push(tokenLBrace, tokenRBrace, start, start+1)
				i++
				if err := lexExpr(); err != nil {
					return err
				}
				continue
			case ')':
				if err := pop(tokenRParen, start, start+1); err != nil {
					return err
				}
				i++
				return nil
			case ']':
				if err := pop(tokenRBracket, start, start+1); err != nil {
					return err
				}
				i++
				return nil
			case '}':
				if len(backrefs) == 0 {
					return r.errorAt(start, "unexpected '}'")
				}
				want := TokenType(backrefs[len(backrefs)-1][1])
				if want != tokenRBrace && want != tokenInterpClose {
					return r.errorAt(start, "unexpected '}'")
				}
				if err := pop(want, start, start+1); err != nil {
					return err
				}
				i++
				return nil
			case '"':
				push(tokenDQuote, tokenDQuote, start, start+1)
				i++
				if err := lexDQuoteString(); err != nil {
					return err
				}
				continue
			case '+':
				emit(tokenPlus, start, start+1)
				i++
				continue
			case '-':
				emit(tokenMinus, start, start+1)
				i++
				continue
			case '*':
				emit(tokenStar, start, start+1)
				i++
				continue
			case '/':
				emit(tokenSlash, start, start+1)
				i++
				continue
			case '!':
				emit(tokenBang, start, start+1)
				i++
				continue
			case '<':
				emit(tokenLt, start, start+1)
				i++
				continue
			case '>':
				emit(tokenGt, start, start+1)
				i++
				continue
			case '?':
				emit(tokenQuestion, start, start+1)
				i++
				continue
			case '=':
				emit(tokenEq, start, start+1)
				i++
				continue
			}

			// URI
			if uriEnd, ok := scanURI(src, i); ok {
				emit(tokenURI, start, uriEnd)
				i = uriEnd
				continue
			}

			// path forms
			if pathEnd, ok := scanPath(src, i); ok {
				emit(tokenPath, start, pathEnd)
				i = pathEnd
				continue
			}
			if pathEnd, ok := scanHPath(src, i); ok {
				emit(tokenPath, start, pathEnd)
				i = pathEnd
				continue
			}
			if pathEnd, ok := scanSPath(src, i); ok {
				emit(tokenPath, start, pathEnd)
				i = pathEnd
				continue
			}

			// float
			if fEnd, ok := scanFloat(src, i); ok {
				emit(tokenFloat, start, fEnd)
				i = fEnd
				continue
			}

			// int
			if intEnd, ok := scanInt(src, i); ok {
				emit(tokenInt, start, intEnd)
				i = intEnd
				continue
			}

			// id / keyword
			if idEnd, ok := scanID(src, i); ok {
				text := string(src[start:idEnd])
				switch text {
				case "assert":
					emit(tokenAssert, start, idEnd)
				case "else":
					emit(tokenElse, start, idEnd)
				case "if":
					emit(tokenIf, start, idEnd)
				case "in":
					emit(tokenIn, start, idEnd)
				case "inherit":
					emit(tokenInherit, start, idEnd)
				case "let":
					emit(tokenLet, start, idEnd)
				case "or":
					emit(tokenOrKW, start, idEnd)
				case "rec":
					emit(tokenRec, start, idEnd)
				case "then":
					emit(tokenThen, start, idEnd)
				case "with":
					emit(tokenWith, start, idEnd)
				default:
					emit(tokenID, start, idEnd)
				}
				i = idEnd
				continue
			}

			rn, _ := utf8.DecodeRune(src[i:])
			return r.errorAt(start, "unexpected character %q", string(rn))
		}
		return nil
	}

	if err := lexExpr(); err != nil {
		return nil, err
	}
	if i < n {
		return nil, r.errorAt(i, "unexpected trailing input")
	}
	if len(backrefs) != 0 {
		br := backrefs[len(backrefs)-1]
		opener := r.tokens[br[0]]
		return nil, r.errorAt(opener.End, "%s is not terminated", opener.Type)
	}
	emit(tokenEOF, n, n)
	return r, nil
}

func scanID(src []byte, i int) (int, bool) {
	if i >= len(src) {
		return 0, false
	}
	c := src[i]
	if !(c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return 0, false
	}
	i++
	for i < len(src) {
		c = src[i]
		if c == '_' || c == '\'' || c == '-' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			i++
			continue
		}
		break
	}
	return i, true
}

func scanInt(src []byte, i int) (int, bool) {
	if i >= len(src) || src[i] < '0' || src[i] > '9' {
		return 0, false
	}
	start := i
	for i < len(src) && src[i] >= '0' && src[i] <= '9' {
		i++
	}
	// not int if followed by '.' that starts float (handled by float scanner first - we call int after float fails)
	if i > start {
		return i, true
	}
	return 0, false
}

func scanFloat(src []byte, i int) (int, bool) {
	start := i
	j := i
	if j < len(src) && src[j] >= '1' && src[j] <= '9' {
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
		if j >= len(src) || src[j] != '.' {
			return 0, false
		}
		j++
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
	} else if j < len(src) && src[j] == '0' && j+1 < len(src) && src[j+1] == '.' {
		j += 2
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
	} else if j < len(src) && src[j] == '.' {
		j++
		if j >= len(src) || src[j] < '0' || src[j] > '9' {
			return 0, false
		}
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
	} else {
		return 0, false
	}
	if j < len(src) && (src[j] == 'e' || src[j] == 'E') {
		j++
		if j < len(src) && (src[j] == '+' || src[j] == '-') {
			j++
		}
		expStart := j
		for j < len(src) && src[j] >= '0' && src[j] <= '9' {
			j++
		}
		if j == expStart {
			return 0, false
		}
	}
	if j > start {
		return j, true
	}
	return 0, false
}

func scanURI(src []byte, i int) (int, bool) {
	if i >= len(src) {
		return 0, false
	}
	c := src[i]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return 0, false
	}
	j := i + 1
	for j < len(src) {
		c = src[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '+' || c == '-' {
			j++
			continue
		}
		break
	}
	if j >= len(src) || src[j] != ':' {
		return 0, false
	}
	j++
	for j < len(src) {
		c = src[j]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '%' || c == '/' || c == '?' || c == ':' || c == '@' || c == '&' || c == '=' ||
			c == '+' || c == '$' || c == ',' || c == '_' || c == '.' || c == '!' || c == '~' ||
			c == '*' || c == '\'' || c == '-' {
			j++
			continue
		}
		break
	}
	if j > i+1 {
		return j, true
	}
	return 0, false
}

func pathChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
		c == '.' || c == '_' || c == '+' || c == '-'
}

func scanPath(src []byte, i int) (int, bool) {
	j := i
	for j < len(src) && pathChar(src[j]) {
		j++
	}
	if j == i {
		return 0, false
	}
	// need at least one /
	k := j
	sawSlash := false
	for k < len(src) {
		if src[k] != '/' {
			break
		}
		sawSlash = true
		k++
		for k < len(src) && pathChar(src[k]) {
			k++
		}
	}
	if !sawSlash {
		return 0, false
	}
	if k < len(src) && src[k-1] != '/' && src[k] == '/' {
		k++
	}
	return k, true
}

func scanHPath(src []byte, i int) (int, bool) {
	if i >= len(src) || src[i] != '~' {
		return 0, false
	}
	j := i + 1
	if j >= len(src) || src[j] != '/' {
		return 0, false
	}
	j++
	for j < len(src) {
		if src[j] != '/' {
			for j < len(src) && pathChar(src[j]) {
				j++
			}
			if j >= len(src) || src[j] != '/' {
				if j < len(src) && src[j] != '/' {
					return j, true
				}
				break
			}
		}
		j++
		for j < len(src) && pathChar(src[j]) {
			j++
		}
	}
	if j > i+1 {
		return j, true
	}
	return 0, false
}

func scanSPath(src []byte, i int) (int, bool) {
	if i >= len(src) || src[i] != '<' {
		return 0, false
	}
	j := i + 1
	for j < len(src) && pathChar(src[j]) {
		j++
	}
	for j < len(src) && src[j] == '/' {
		j++
		for j < len(src) && pathChar(src[j]) {
			j++
		}
	}
	if j < len(src) && src[j] == '>' {
		return j + 1, true
	}
	return 0, false
}
