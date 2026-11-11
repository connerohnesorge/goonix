package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBuiltinRegexLeftmostLongestOracle(t *testing.T) {
	cases := []string{
		`builtins.toJSON (builtins.match "a|ab" "ab")`,
		`builtins.toJSON (builtins.match "x*|xy" "xy")`,
		`builtins.toJSON (builtins.split "a|ab" "xabx")`,
		`builtins.toJSON (builtins.split "a*" "baaac")`,
		`builtins.toJSON (builtins.match "(a)|(b)" "a")`,
		`builtins.toJSON (builtins.match "(a)|(b)" "x")`,
		`builtins.toString (builtins.match "a.b" "a\nb" != null)`,
	}
	lay := oracle.MustLayout(t)
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if want != got {
				t.Fatalf("oracle mismatch\nnix=%q\ngo=%q", want, got)
			}
		})
	}
}

func TestBuiltinRegexRejectsExtensionsOracle(t *testing.T) {
	cases := []struct {
		re string
		s  string
	}{
		{`\d+`, "1"},
		{`\bword`, "word"},
		{"(?i)abc", "ABC"},
	}
	for _, tc := range cases {
		t.Run(tc.re, func(t *testing.T) {
			_, err := evalPkgsExprStringAllowError(t, `builtins.match `+quoteNixString(tc.re)+` "x"`)
			if err == nil {
				t.Fatal("expected invalid regex error")
			}
		})
	}
}

func quoteNixString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func TestWithScopingBuiltinsBeatWithOracle(t *testing.T) {
	assertExprMatchesNix(t, `builtins.toString (with { map = 1; }; builtins.isInt map)`)
}

func TestBuiltinHashStringOracle(t *testing.T) {
	cases := []string{
		`builtins.hashString "sha256" "foo"`,
		`builtins.hashString "md5" "foo"`,
		`builtins.hashString "sha1" "foo"`,
		`builtins.hashString "sha512" "foo"`,
	}
	lay := oracle.MustLayout(t)
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if want != got {
				t.Fatalf("oracle mismatch\nnix=%q\ngo=%q", want, got)
			}
		})
	}
}
