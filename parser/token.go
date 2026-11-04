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

// TokenType identifies a lexical token in the Nix language.
type TokenType int

const (
	tokenEOF TokenType = iota
	tokenIllegal

	// Keywords
	tokenAssert
	tokenElse
	tokenIf
	tokenIn
	tokenInherit
	tokenLet
	tokenOrKW
	tokenRec
	tokenThen
	tokenWith

	// Literals
	tokenInt
	tokenFloat
	tokenPath
	tokenURI
	tokenID
	tokenText
	tokenEllipsis

	// String / indented string delimiters
	tokenDQuote
	tokenIStringOpen
	tokenIStringClose
	tokenInterpOpen
	tokenInterpClose

	// Function-argument markers (after @ or : reclassification)
	tokenArgID
	tokenArgBracket

	// Single-char and multi-char punctuation
	tokenColon
	tokenAt
	tokenComma
	tokenSemi
	tokenDot
	tokenLParen
	tokenRParen
	tokenLBracket
	tokenRBracket
	tokenLBrace
	tokenRBrace
	tokenEq

	// Operators
	tokenImpl   // ->
	tokenOr       // ||
	tokenAnd      // &&
	tokenEqEq     // ==
	tokenNotEq    // !=
	tokenLeq      // <=
	tokenGeq      // >=
	tokenUpdate   // //
	tokenConcat   // ++
	tokenPlus
	tokenMinus
	tokenStar
	tokenSlash
	tokenBang
	tokenLt
	tokenGt
	tokenQuestion

	tokenComment
)

func (t TokenType) String() string {
	switch t {
	case tokenEOF:
		return "EOF"
	case tokenAssert:
		return "assert"
	case tokenElse:
		return "else"
	case tokenIf:
		return "if"
	case tokenIn:
		return "in"
	case tokenInherit:
		return "inherit"
	case tokenLet:
		return "let"
	case tokenOrKW:
		return "or"
	case tokenRec:
		return "rec"
	case tokenThen:
		return "then"
	case tokenWith:
		return "with"
	case tokenInt:
		return "int"
	case tokenFloat:
		return "float"
	case tokenPath:
		return "path"
	case tokenURI:
		return "uri"
	case tokenID:
		return "id"
	case tokenText:
		return "text"
	case tokenEllipsis:
		return "..."
	case tokenDQuote:
		return `"'`
	case tokenIStringOpen, tokenIStringClose:
		return "''"
	case tokenInterpOpen:
		return "${"
	case tokenInterpClose:
		return "}"
	case tokenArgID:
		return "argID"
	case tokenArgBracket:
		return "argBracket"
	case tokenColon:
		return ":"
	case tokenAt:
		return "@"
	case tokenComma:
		return ","
	case tokenSemi:
		return ";"
	case tokenDot:
		return "."
	case tokenLParen:
		return "("
	case tokenRParen:
		return ")"
	case tokenLBracket:
		return "["
	case tokenRBracket:
		return "]"
	case tokenLBrace:
		return "{"
	case tokenRBrace:
		return "}"
	case tokenEq:
		return "="
	case tokenImpl:
		return "->"
	case tokenOr:
		return "||"
	case tokenAnd:
		return "&&"
	case tokenEqEq:
		return "=="
	case tokenNotEq:
		return "!="
	case tokenLeq:
		return "<="
	case tokenGeq:
		return ">="
	case tokenUpdate:
		return "//"
	case tokenConcat:
		return "++"
	case tokenPlus:
		return "+"
	case tokenMinus:
		return "-"
	case tokenStar:
		return "*"
	case tokenSlash:
		return "/"
	case tokenBang:
		return "!"
	case tokenLt:
		return "<"
	case tokenGt:
		return ">"
	case tokenQuestion:
		return "?"
	case tokenComment:
		return "comment"
	default:
		return "unknown"
	}
}

// Token is a lexical token with source span.
type Token struct {
	Type  TokenType
	Pos   Pos
	Start int // byte offset in source (inclusive)
	End   int // byte offset in source (exclusive)
	// Prev links a closing delimiter to its opener index in the token stream.
	Prev int
}
