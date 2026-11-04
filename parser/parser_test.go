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
	"os"
	"strings"
	"testing"
)

func TestParseInteger(t *testing.T) {
	expr, err := ParseString("42")
	if err != nil {
		t.Fatal(err)
	}
	if got := Print(expr); got != "42" {
		t.Fatalf("Print = %q, want 42", got)
	}
}

func TestParseOperators(t *testing.T) {
	expr, err := ParseString("1 + 2 * 3")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	want := "(1 + (mul 2 3))"
	if got != want {
		t.Fatalf("Print = %q, want %q", got, want)
	}
}

func TestParsePrecedenceUpdate(t *testing.T) {
	expr, err := ParseString("{ a = 1; } // { b = 2; }")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if !strings.Contains(got, "//") {
		t.Fatalf("Print = %q, expected // operator", got)
	}
}

func TestParseLambda(t *testing.T) {
	expr, err := ParseString("{ lib }: lib")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	want := "({ lib }: lib)"
	if got != want {
		t.Fatalf("Print = %q, want %q", got, want)
	}
}

func TestParseRecInherit(t *testing.T) {
	src := `
rec {
  inherit x;
  y = 1;
}
`
	expr, err := ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if !strings.Contains(got, "rec {") || !strings.Contains(got, "inherit x") {
		t.Fatalf("Print = %q", got)
	}
}

func TestParseStringInterpolation(t *testing.T) {
	expr, err := ParseString(`"a${"b"}c"`)
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if !strings.Contains(got, "+") {
		t.Fatalf("Print = %q", got)
	}
}

func TestParseIfLetWith(t *testing.T) {
	cases := []string{
		"if true then 1 else 2",
		"let a = 1; in a",
		"with {}; 1",
		"assert true; 1",
	}
	for _, src := range cases {
		if _, err := ParseString(src); err != nil {
			t.Fatalf("parse %q: %v", src, err)
		}
	}
}

func TestParseIndentedString(t *testing.T) {
	expr, err := ParseString("''hello''")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if got != `"hello"` {
		t.Fatalf("Print = %q", got)
	}
}

func TestParseHasAttr(t *testing.T) {
	expr, err := ParseString("foo ? bar")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	want := "((foo) ? bar)"
	if got != want {
		t.Fatalf("Print = %q, want %q", got, want)
	}
}

func TestParseSelectOr(t *testing.T) {
	expr, err := ParseString("a.b or 0")
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	if !strings.Contains(got, " or (") {
		t.Fatalf("Print = %q", got)
	}
}

func TestLexCommentsSkipped(t *testing.T) {
	expr, err := ParseString("# comment\n1")
	if err != nil {
		t.Fatal(err)
	}
	if Print(expr) != "1" {
		t.Fatalf("got %q", Print(expr))
	}
}

func TestParseLetImportPath(t *testing.T) {
	src := `let requiredVersion = import ./lib/minver.nix; in requiredVersion`
	if _, err := ParseString(src); err != nil {
		t.Fatal(err)
	}
}

func TestParseBuiltinsHasAttr(t *testing.T) {
	expr, err := ParseString(`!builtins ? nixVersion`)
	if err != nil {
		t.Fatal(err)
	}
	got := Print(expr)
	want := "(! ((builtins) ? nixVersion))"
	if got != want {
		t.Fatalf("Print = %q, want %q", got, want)
	}
}

func TestParseCompareVersionsExpr(t *testing.T) {
	src := `builtins.compareVersions requiredVersion builtins.nixVersion`
	if _, err := ParseString(src); err != nil {
		t.Fatal(err)
	}
}

func TestParseMultilineIndentedString(t *testing.T) {
	src := "abort ''\n  line one\n  line two\n''"
	if _, err := ParseString(src); err != nil {
		t.Fatal(err)
	}
}

func TestParseNixpkgsDefaultNix(t *testing.T) {
	b, err := os.ReadFile("../testdata/nixpkgs-default-head.nix")
	if err != nil {
		t.Skip(err)
	}
	if _, err := ParseString(string(b)); err != nil {
		t.Fatal(err)
	}
}
