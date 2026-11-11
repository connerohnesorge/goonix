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
	"math"
	"os"
	"strconv"
)

// ParseString parses a Nix expression from source text.
func ParseString(src string) (Expr, error) {
	lr, err := lex([]byte(src), "")
	if err != nil {
		return nil, err
	}
	return parse(lr)
}

// ParseFile parses a Nix expression from a file.
func ParseFile(path string) (Expr, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lr, err := lex(src, path)
	if err != nil {
		return nil, err
	}
	return parse(lr)
}

func parse(lr *lexResult) (Expr, error) {
	p := &Parser{lr: lr}
	expr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if p.cur().Type != tokenEOF {
		return nil, p.errorf("unexpected token %s", p.cur().Type)
	}
	return expr, nil
}

// Parser is a recursive-descent parser over a token stream.
type Parser struct {
	lr *lexResult
	i  int
}

func (p *Parser) cur() Token {
	if p.i >= len(p.lr.tokens) {
		return Token{Type: tokenEOF}
	}
	return p.lr.tokens[p.i]
}

func (p *Parser) advance() Token {
	t := p.cur()
	if p.i < len(p.lr.tokens) {
		p.i++
	}
	return t
}

func (p *Parser) peek() Token {
	return p.cur()
}

func (p *Parser) text(t Token) string {
	return p.lr.slice(t.Start, t.End)
}

// nixFloatLiteralOK rejects values outside the normal float64 range. Nix
// reports those literals as invalid, including 5.0e-324.
func nixFloatLiteralOK(text string) bool {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return false
	}
	if f == 0 {
		return true
	}
	const minNormal = 2.2250738585072014e-308
	abs := math.Abs(f)
	return abs >= minNormal && abs <= math.MaxFloat64
}

func (p *Parser) errorf(format string, args ...interface{}) error {
	t := p.cur()
	return fmt.Errorf("%s:%d:%d: %s", t.Pos.File, t.Pos.Line, t.Pos.Col, fmt.Sprintf(format, args...))
}

func (p *Parser) expect(typ TokenType) (Token, error) {
	t := p.cur()
	if t.Type != typ {
		return t, p.errorf("expected %s, got %s", typ, t.Type)
	}
	return p.advance(), nil
}

func (p *Parser) expectArgName() (Token, error) {
	t := p.cur()
	if t.Type != tokenID && t.Type != tokenArgID {
		return t, p.errorf("expected id, got %s", t.Type)
	}
	return p.advance(), nil
}

func (p *Parser) parseExpression() (Expr, error) {
	return p.parseExprFunction()
}

func (p *Parser) parseExprFunction() (Expr, error) {
	switch p.cur().Type {
	case tokenAssert:
		return p.parseAssert()
	case tokenIf:
		return p.parseIf()
	case tokenLet:
		return p.parseLet()
	case tokenWith:
		return p.parseWith()
	case tokenID, tokenArgID, tokenArgBracket, tokenLBrace:
		fn, ok, err := p.tryParseFunction()
		if err != nil {
			return nil, err
		}
		if ok {
			return fn, nil
		}
	}
	return p.parseExprOp()
}

func (p *Parser) parseAssert() (Expr, error) {
	pos := p.advance().Pos
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenSemi); err != nil {
		return nil, err
	}
	body, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &Assert{P: pos, Cond: cond, Body: body}, nil
}

func (p *Parser) parseIf() (Expr, error) {
	pos := p.advance().Pos
	cond, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenThen); err != nil {
		return nil, err
	}
	then, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenElse); err != nil {
		return nil, err
	}
	elseExpr, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &If{P: pos, Cond: cond, Then: then, Else: elseExpr}, nil
}

func (p *Parser) parseLet() (Expr, error) {
	pos := p.advance().Pos
	binds, err := p.parseLetBinds()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenIn); err != nil {
		return nil, err
	}
	body, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &Let{P: pos, Attrs: binds, Body: body}, nil
}

func (p *Parser) parseLetBinds() (*AttrSet, error) {
	set := &AttrSet{P: p.cur().Pos}
	for p.cur().Type != tokenIn && p.cur().Type != tokenEOF {
		if p.cur().Type == tokenInherit {
			inh, err := p.parseInherit()
			if err != nil {
				return nil, err
			}
			inhc := *inh
			set.Inherit = append(set.Inherit, inhc)
			set.Members = append(set.Members, AttrSetMember{Inherit: &inhc})
			continue
		}
		path, err := p.parseAttrPath()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenEq); err != nil {
			return nil, err
		}
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenSemi); err != nil {
			return nil, err
		}
		bind := Binding{
			Pos:   path[0].pos(),
			Path:  path,
			Value: val,
			Kind:  BindPlain,
		}
		set.Binds = append(set.Binds, bind)
		b := bind
		set.Members = append(set.Members, AttrSetMember{Binding: &b})
	}
	return set, nil
}

func (p *Parser) parseWith() (Expr, error) {
	pos := p.advance().Pos
	attrs, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenSemi); err != nil {
		return nil, err
	}
	body, err := p.parseExpression()
	if err != nil {
		return nil, err
	}
	return &With{P: pos, Attrs: attrs, Body: body}, nil
}

func (p *Parser) tryParseFunction() (Expr, bool, error) {
	saved := p.i

	// id @ { formals } : expr
	if p.cur().Type == tokenID || p.cur().Type == tokenArgID {
		argTok := p.advance()
		if p.cur().Type == tokenAt {
			p.advance()
			formals, err := p.parseArgSet()
			if err != nil {
				return nil, true, err
			}
			if _, err := p.expect(tokenColon); err != nil {
				return nil, true, err
			}
			body, err := p.parseExpression()
			if err != nil {
				return nil, true, err
			}
			return &Lambda{P: argTok.Pos, Arg: p.text(argTok), Formals: formals, Body: body, ArgFirst: true}, true, nil
		}
		p.i = saved
	}

	// id : expr
	if p.cur().Type == tokenID || p.cur().Type == tokenArgID {
		argTok := p.advance()
		if p.cur().Type == tokenColon {
			p.advance()
			body, err := p.parseExpression()
			if err != nil {
				return nil, true, err
			}
			return &Lambda{P: argTok.Pos, Arg: p.text(argTok), Body: body}, true, nil
		}
		p.i = saved
	}

	// { formals } : expr  OR  { formals } @ id : expr
	if p.cur().Type == tokenLBrace || p.cur().Type == tokenArgBracket {
		formals, err := p.parseArgSet()
		if err != nil {
			p.i = saved
			return nil, false, nil
		}
		if p.cur().Type == tokenAt {
			p.advance()
			argTok, err := p.expectArgName()
			if err != nil {
				return nil, true, err
			}
			if _, err := p.expect(tokenColon); err != nil {
				return nil, true, err
			}
			body, err := p.parseExpression()
			if err != nil {
				return nil, true, err
			}
			return &Lambda{P: formals.P, Formals: formals, Arg: p.text(argTok), Body: body, ArgFirst: false}, true, nil
		}
		if p.cur().Type == tokenColon {
			p.advance()
			body, err := p.parseExpression()
			if err != nil {
				return nil, true, err
			}
			return &Lambda{P: formals.P, Formals: formals, Body: body}, true, nil
		}
		p.i = saved
	}
	return nil, false, nil
}

func (p *Parser) parseArgSet() (*Formals, error) {
	open, err := p.expect(p.cur().Type)
	if err != nil {
		return nil, err
	}
	if open.Type != tokenLBrace && open.Type != tokenArgBracket {
		return nil, p.errorf("expected argument set")
	}
	f := &Formals{P: open.Pos}
	for {
		switch p.cur().Type {
		case tokenRBrace:
			p.advance()
			return f, nil
		case tokenEllipsis:
			p.advance()
			f.Ellipsis = true
			if _, err := p.expect(tokenRBrace); err != nil {
				return nil, err
			}
			return f, nil
		case tokenID, tokenOrKW:
			arg, err := p.parseFormal()
			if err != nil {
				return nil, err
			}
			f.Params = append(f.Params, *arg)
			if p.cur().Type == tokenComma {
				p.advance()
				continue
			}
			if p.cur().Type == tokenEllipsis {
				p.advance()
				f.Ellipsis = true
				if _, err := p.expect(tokenRBrace); err != nil {
					return nil, err
				}
				return f, nil
			}
			if _, err := p.expect(tokenRBrace); err != nil {
				return nil, err
			}
			return f, nil
		default:
			return nil, p.errorf("unexpected token %s in argument set", p.cur().Type)
		}
	}
}

func (p *Parser) parseFormal() (*Formal, error) {
	var nameTok Token
	switch p.cur().Type {
	case tokenID:
		nameTok = p.advance()
	case tokenOrKW:
		nameTok = p.advance()
	default:
		return nil, p.errorf("expected formal parameter name")
	}
	f := &Formal{P: nameTok.Pos, Name: p.text(nameTok)}
	if p.cur().Type == tokenQuestion {
		p.advance()
		def, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		f.Default = def
	}
	return f, nil
}

// Operator precedence (lower number = looser binding).
const (
	precImpl = iota
	precOr
	precAnd
	precEq
	precRel
	precUpdate
	precNot
	precAdd
	precMul
	precConcat
	precHas
	precUnary
)

func (p *Parser) parseExprOp() (Expr, error) {
	return p.parseBinOp(precImpl)
}

func (p *Parser) parseBinOp(prec int) (Expr, error) {
	unaryPrec := prec
	if prec < precUnary {
		unaryPrec = precUnary
	}
	left, err := p.parseUnary(unaryPrec)
	if err != nil {
		return nil, err
	}
	for {
		op, opPrec, err := p.peekBinOp()
		if err != nil {
			return nil, err
		}
		if op == "" || opPrec < prec {
			return left, nil
		}
		if op == "?" {
			p.advance()
			path, err := p.parseAttrPath()
			if err != nil {
				return nil, err
			}
			left = &HasAttr{P: left.Pos(), Expr: left, Path: path}
			continue
		}
		p.advance()
		right, err := p.parseBinOp(opPrec + 1)
		if err != nil {
			return nil, err
		}
		if op == "+" {
			left = &ConcatStrings{P: left.Pos(), Parts: []Expr{left, right}}
			continue
		}
		if op == "-" || op == "*" || op == "/" {
			builtin := "sub"
			if op == "*" {
				builtin = "mul"
			} else if op == "/" {
				builtin = "div"
			}
			left = &Call{
				P:    left.Pos(),
				Fun:  &Var{P: left.Pos(), Name: builtin},
				Args: []Expr{left, right},
			}
			continue
		}
		left = &BinaryOp{P: left.Pos(), Op: op, Left: left, Right: right}
	}
}

func (p *Parser) peekBinOp() (string, int, error) {
	switch p.cur().Type {
	case tokenImpl:
		return "->", precImpl, nil
	case tokenOr:
		return "||", precOr, nil
	case tokenAnd:
		return "&&", precAnd, nil
	case tokenEqEq:
		return "==", precEq, nil
	case tokenNotEq:
		return "!=", precEq, nil
	case tokenLt, tokenGt, tokenLeq, tokenGeq:
		return p.text(p.cur()), precRel, nil
	case tokenUpdate:
		return "//", precUpdate, nil
	case tokenPlus:
		return "+", precAdd, nil
	case tokenMinus:
		return "-", precAdd, nil
	case tokenStar:
		return "*", precMul, nil
	case tokenSlash:
		return "/", precMul, nil
	case tokenConcat:
		return "++", precConcat, nil
	case tokenQuestion:
		return "?", precHas, nil
	default:
		return "", 0, nil
	}
}

func (p *Parser) parseUnary(prec int) (Expr, error) {
	if p.cur().Type == tokenMinus && prec >= precUnary {
		pos := p.advance().Pos
		inner, err := p.parseUnary(precUnary)
		if err != nil {
			return nil, err
		}
		return &UnaryOp{P: pos, Op: "-", Expr: inner}, nil
	}
	if p.cur().Type == tokenBang {
		pos := p.advance().Pos
		inner, err := p.parseBinOp(precNot + 1)
		if err != nil {
			return nil, err
		}
		return &UnaryOp{P: pos, Op: "!", Expr: inner}, nil
	}
	return p.parseApply()
}

func (p *Parser) parseApply() (Expr, error) {
	left, err := p.parseSelect()
	if err != nil {
		return nil, err
	}
	for {
		if !p.isSelectStart() {
			break
		}
		arg, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		if call, ok := left.(*Call); ok {
			call.Args = append(call.Args, arg)
			left = call
		} else {
			left = &Call{P: left.Pos(), Fun: left, Args: []Expr{arg}}
		}
	}
	return left, nil
}

func (p *Parser) isSelectStart() bool {
	switch p.cur().Type {
	case tokenURI, tokenPath, tokenFloat, tokenInt, tokenID, tokenOrKW,
		tokenDQuote, tokenIStringOpen, tokenLParen, tokenLBracket, tokenLBrace,
		tokenRec, tokenAssert, tokenIf, tokenLet, tokenWith:
		return true
	default:
		return false
	}
}

func (p *Parser) parseSelect() (Expr, error) {
	left, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	for p.cur().Type == tokenDot {
		p.advance()
		path, err := p.parseAttrPath()
		if err != nil {
			return nil, err
		}
		if p.cur().Type == tokenOrKW {
			p.advance()
			def, err := p.parseSelect()
			if err != nil {
				return nil, err
			}
			left = &SelectOr{P: left.Pos(), Expr: left, Path: path, Default: def}
		} else {
			left = &Select{P: left.Pos(), Expr: left, Path: path}
		}
	}
	if p.cur().Type == tokenOrKW {
		p.advance()
		left = &Call{
			P:    left.Pos(),
			Fun:  left,
			Args: []Expr{&Var{P: left.Pos(), Name: "or"}},
		}
	}
	return left, nil
}

func (p *Parser) parseAtom() (Expr, error) {
	switch p.cur().Type {
	case tokenURI:
		t := p.advance()
		return &URI{P: t.Pos, Text: p.text(t)}, nil
	case tokenPath:
		t := p.advance()
		return &Path{P: t.Pos, Text: p.text(t)}, nil
	case tokenFloat:
		t := p.cur()
		text := p.text(t)
		if !nixFloatLiteralOK(text) {
			return nil, fmt.Errorf("%s:%d:%d: invalid float '%s'", t.Pos.File, t.Pos.Line, t.Pos.Col, text)
		}
		p.advance()
		return &Float{P: t.Pos, Text: text}, nil
	case tokenInt:
		t := p.advance()
		return &Integer{P: t.Pos, Text: p.text(t)}, nil
	case tokenID:
		t := p.advance()
		if p.text(t) == "__curPos" {
			return &CurPos{P: t.Pos}, nil
		}
		return &Var{P: t.Pos, Name: p.text(t)}, nil
	case tokenOrKW:
		t := p.advance()
		return &Var{P: t.Pos, Name: "or"}, nil
	case tokenDQuote:
		return p.parseQuotedString()
	case tokenIStringOpen:
		return p.parseIndentedString()
	case tokenLParen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
		return &Parens{P: expr.Pos(), Expr: expr}, nil
	case tokenLBracket:
		return p.parseList()
	case tokenRec:
		pos := p.advance().Pos
		if _, err := p.expect(tokenLBrace); err != nil {
			return nil, err
		}
		binds, err := p.parseBindsInner()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRBrace); err != nil {
			return nil, err
		}
		binds.Recursive = true
		binds.P = pos
		return binds, nil
	case tokenLBrace:
		pos := p.advance().Pos
		binds, err := p.parseBindsInner()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRBrace); err != nil {
			return nil, err
		}
		binds.P = pos
		return binds, nil
	default:
		return nil, p.errorf("unexpected token %s", p.cur().Type)
	}
}

func (p *Parser) parseList() (Expr, error) {
	open := p.advance()
	list := &List{P: open.Pos}
	for p.cur().Type != tokenRBracket && p.cur().Type != tokenEOF {
		elem, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		list.Elems = append(list.Elems, elem)
	}
	if _, err := p.expect(tokenRBracket); err != nil {
		return nil, err
	}
	return list, nil
}

func (p *Parser) parseQuotedString() (Expr, error) {
	open := p.advance()
	parts, err := p.parseStringParts(tokenDQuote)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenDQuote); err != nil {
		return nil, err
	}
	return &String{P: open.Pos, Parts: parts}, nil
}

func (p *Parser) parseIndentedString() (Expr, error) {
	open := p.advance()
	parts, err := p.parseIndStringParts()
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(tokenIStringClose); err != nil {
		return nil, err
	}
	return stripIndentation(open.Pos, parts), nil
}

func (p *Parser) parseIndStringParts() ([]indStringPart, error) {
	var parts []indStringPart
	for p.cur().Type != tokenIStringClose && p.cur().Type != tokenEOF {
		switch p.cur().Type {
		case tokenISText:
			t := p.advance()
			raw := p.text(t)
			text, indent := decodeIndStringText(raw, true)
			parts = append(parts, indStringPart{pos: t.Pos, text: text, hasIndent: indent})
		case tokenISPlain:
			t := p.advance()
			raw := p.lr.slice(t.Start, t.End)
			text := raw
			if len(raw) >= 2 && raw[0] == '\'' && raw[1] == '\'' {
				text, _ = decodeIndStringEscape([]byte(raw), 0)
			}
			parts = append(parts, indStringPart{pos: t.Pos, text: text, hasIndent: false})
		case tokenInterpOpen:
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokenInterpClose); err != nil {
				return nil, err
			}
			parts = append(parts, indStringPart{pos: expr.Pos(), interp: expr})
		default:
			return nil, p.errorf("unexpected token %s in indented string", p.cur().Type)
		}
	}
	return parts, nil
}

func (p *Parser) parseStringParts(close TokenType) ([]StringPart, error) {
	var parts []StringPart
	for p.cur().Type != close && p.cur().Type != tokenEOF {
		switch p.cur().Type {
		case tokenText:
			t := p.advance()
			parts = append(parts, StringPart{Pos: t.Pos, Text: decodeRegularStringText(p.text(t))})
		case tokenInterpOpen:
			p.advance()
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if _, err := p.expect(tokenInterpClose); err != nil {
				return nil, err
			}
			parts = append(parts, StringPart{Pos: expr.Pos(), Interp: expr})
		default:
			return nil, p.errorf("unexpected token %s in string", p.cur().Type)
		}
	}
	return parts, nil
}

func (p *Parser) parseBinds() (*AttrSet, error) {
	return p.parseBindsInner()
}

func (p *Parser) parseBindsInner() (*AttrSet, error) {
	set := &AttrSet{P: p.cur().Pos}
	for p.cur().Type != tokenRBrace && p.cur().Type != tokenEOF {
		if p.cur().Type == tokenInherit {
			inh, err := p.parseInherit()
			if err != nil {
				return nil, err
			}
			inhc := *inh
			set.Inherit = append(set.Inherit, inhc)
			set.Members = append(set.Members, AttrSetMember{Inherit: &inhc})
			continue
		}
		path, err := p.parseAttrPath()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenEq); err != nil {
			return nil, err
		}
		val, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenSemi); err != nil {
			return nil, err
		}
		bind := Binding{
			Pos:   path[0].pos(),
			Path:  path,
			Value: val,
			Kind:  BindPlain,
		}
		set.Binds = append(set.Binds, bind)
		b := bind
		set.Members = append(set.Members, AttrSetMember{Binding: &b})
	}
	return set, nil
}

func (p *Parser) parseInherit() (*Inherit, error) {
	pos := p.advance().Pos
	inh := &Inherit{Pos: pos}
	if p.cur().Type == tokenLParen {
		p.advance()
		from, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRParen); err != nil {
			return nil, err
		}
		inh.From = from
	}
	names, err := p.parseInheritList()
	if err != nil {
		return nil, err
	}
	inh.Names = names
	if _, err := p.expect(tokenSemi); err != nil {
		return nil, err
	}
	return inh, nil
}

func (p *Parser) parseInheritList() ([]AttrName, error) {
	var names []AttrName
	for p.cur().Type != tokenSemi && p.cur().Type != tokenRBrace && p.cur().Type != tokenEOF {
		name, err := p.parseInterpID()
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func (p *Parser) parseAttrPath() ([]AttrName, error) {
	first, err := p.parseInterpID()
	if err != nil {
		return nil, err
	}
	path := []AttrName{first}
	for p.cur().Type == tokenDot {
		p.advance()
		seg, err := p.parseInterpID()
		if err != nil {
			return nil, err
		}
		path = append(path, seg)
	}
	return path, nil
}

func (p *Parser) parseInterpID() (AttrName, error) {
	switch p.cur().Type {
	case tokenID, tokenOrKW:
		t := p.advance()
		return AttrName{Name: p.text(t), Pos: t.Pos}, nil
	case tokenDQuote:
		qpos := p.cur().Pos
		p.advance()
		parts, err := p.parseStringParts(tokenDQuote)
		if err != nil {
			return AttrName{}, err
		}
		if _, err := p.expect(tokenDQuote); err != nil {
			return AttrName{}, err
		}
		if len(parts) == 1 && parts[0].Interp == nil {
			return AttrName{Name: parts[0].Text, Pos: qpos}, nil
		}
		return AttrName{Expr: stringExpr(parts[0].Pos, parts)}, nil
	case tokenInterpOpen:
		p.advance()
		expr, err := p.parseExpression()
		if err != nil {
			return AttrName{}, err
		}
		if _, err := p.expect(tokenInterpClose); err != nil {
			return AttrName{}, err
		}
		return AttrName{Expr: expr}, nil
	default:
		return AttrName{}, p.errorf("expected attribute name")
	}
}

func stringExpr(pos Pos, parts []StringPart) Expr {
	if len(parts) == 1 && parts[0].Interp == nil {
		return &String{P: pos, Parts: parts}
	}
	var exprs []Expr
	for _, part := range parts {
		if part.Interp != nil {
			exprs = append(exprs, part.Interp)
		} else if part.Text != "" {
			exprs = append(exprs, &String{P: part.Pos, Parts: []StringPart{{Pos: part.Pos, Text: part.Text}}})
		}
	}
	if len(exprs) == 1 {
		return exprs[0]
	}
	return &ConcatStrings{P: pos, Parts: exprs}
}

func (a AttrName) pos() Pos {
	if a.Pos.Line != 0 || a.Pos.File != "" {
		return a.Pos
	}
	if a.Expr != nil {
		return a.Expr.Pos()
	}
	return Pos{}
}
