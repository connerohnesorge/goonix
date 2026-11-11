package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// TestNixSemantics pins evaluator behaviour that differed from Nix 2.24.9.
// Each want is builtins.toJSON of expr as nix-instantiate --eval prints it
// (without the outer quoting), or, for errors, a substring of Nix's message.
func TestNixSemantics(t *testing.T) {
	cases := []struct {
		name, expr, want, wantErr string
	}{
		// $$ is text in both string kinds, so $${ does not interpolate.
		{name: "dollar-dollar-brace", expr: `"$${a}"`, want: `"$${a}"`},
		{name: "dollar-dollar-brace-indented", expr: `''$${a}''`, want: `"$${a}"`},
		{name: "four-dollars", expr: `"$$$${a}"`, want: `"$$$${a}"`},
		{name: "three-dollars-interp", expr: `let a = "A"; in "$$${a}"`, want: `"$$A"`},
		// An indented string of only blank lines keeps the newlines.
		{name: "indented-blank-lines", expr: "''\n\n  ''", want: `"\n"`},
		{name: "indented-whitespace-line", expr: "''\n  \n  ''", want: `"\n"`},
		// Attribute paths desugar the way Nix's addAttr does, also in let.
		{name: "let-attrpath", expr: `let a.b = 1; a.c = 2; in a`, want: `{"b":1,"c":2}`},
		{name: "let-merge-literal", expr: `let a = { x = 1; }; a.y = 2; in a`, want: `{"x":1,"y":2}`},
		{name: "set-merge-literals", expr: `{ a = { x = 1; }; a = { y = 2; }; }`, want: `{"a":{"x":1,"y":2}}`},
		{name: "empty-attr-name", expr: `{ "" = 1; }`, want: `{"":1}`},
		{name: "duplicate-attr", expr: `{ a = 1; a = 2; }`, wantErr: "attribute 'a' already defined"},
		{name: "duplicate-inherit", expr: `let a = 1; in { inherit a; a = 2; }`, wantErr: "attribute 'a' already defined"},
		{name: "dynamic-in-let", expr: `let x = "a"; ${x} = 1; in 1`, wantErr: "dynamic attributes not allowed in let"},
		// Arithmetic desugars to __sub, __mul and __div, which can be shadowed.
		{name: "user-sub-ignored", expr: `let sub = a: b: 42; in 5 - 3`, want: `2`},
		{name: "dunder-sub-shadowed", expr: `let __sub = a: b: 7; in 5 - 3`, want: `7`},
		// Only Nix's global builtins are bare names, so with can supply the rest.
		{name: "with-fetchurl", expr: `with { fetchurl = "W"; }; fetchurl`, want: `"W"`},
		{name: "with-map-is-global", expr: `with { map = "W"; }; builtins.isFunction map`, want: `true`},
		{name: "dunder-alias", expr: `__length [ 1 2 ]`, want: `2`},
		// let inherit x; reads x from the enclosing scope.
		{name: "let-inherit-outer", expr: `let lib = 5; in let inherit lib; in lib`, want: `5`},
		// -> is right-associative.
		{name: "implication-right-assoc", expr: `[ (false -> true -> false) (true -> false -> false) ]`, want: `[true,true]`},
		// + is decided by its first operand.
		{name: "int-plus-float", expr: `[ (1 + 0.5) (0.1 + 0.2) (1 + 2 + 3.5) ]`, want: `[1.5,0.30000000000000004,6.5]`},
		{name: "string-plus-int", expr: `"a" + 1`, wantErr: "cannot coerce an integer to a string"},
		{name: "int-plus-string", expr: `1 + "a"`, wantErr: "cannot add a string to an integer"},
		{name: "null-plus-int", expr: `null + 1`, wantErr: "cannot coerce null to a string"},
		{name: "interp-int", expr: `''${1}''`, wantErr: "cannot coerce an integer to a string"},
		// Division takes floats and reports overflow.
		{name: "float-division", expr: `[ (2 / 2.0) (7.0 / 2) (builtins.div 7 2.0) (7 / 2) ]`, want: `[1.0,3.5,3.5,3]`},
		{name: "float-division-by-zero", expr: `1.0 / 0`, wantErr: "division by zero"},
		{name: "division-overflow", expr: `(-9223372036854775807 - 1) / (-1)`, wantErr: "overflow in integer division"},
		// == compares int and float numerically, and derivations by outPath.
		{name: "int-float-equal", expr: `[ (1 == 1.0) ([ 1 ] == [ 1.0 ]) ({ a = 1; } == { a = 1.0; }) ]`, want: `[true,true,true]`},
		{name: "derivation-equal-by-outpath", expr: `let mk = x: { type = "derivation"; outPath = "/nix/store/x"; } // x; in [ (mk { foo = 1; } == mk { foo = 2; }) (mk { } == { type = "derivation"; outPath = "/nix/store/y"; }) ]`, want: `[true,false]`},
		// Builtins pass values to f unevaluated.
		{name: "mapAttrs-lazy-value", expr: `(builtins.mapAttrs (n: v: { x = v; }) { etc = { "f-${throw "forced"}" = 1; }; }).etc ? x`, want: `true`},
		{name: "zipAttrsWith-lazy-values", expr: `builtins.zipAttrsWith (n: vs: builtins.length vs) [ { a = throw "x"; } { a = 1; } ]`, want: `{"a":2}`},
		// addErrorContext evaluates its message only on error.
		{name: "addErrorContext-lazy-message", expr: `builtins.addErrorContext (throw "message forced") 1`, want: `1`},
		{name: "addErrorContext-on-error", expr: `builtins.addErrorContext "while testing" (throw "boom")`, wantErr: "while testing"},
		// toJSON does not HTML-escape.
		{name: "toJSON-no-html-escape", expr: `{ a = "x > y < z & w"; }`, want: `{"a":"x > y < z & w"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := eval.NewEvaluator()
			expr, err := parser.ParseString("builtins.toJSON (" + c.expr + "\n)")
			var got string
			if err == nil {
				var v *eval.Value
				v, err = ev.EvalWithBuiltins(expr)
				if err == nil {
					v, err = ev.ForceValue(v)
					if err == nil {
						got = v.String
					}
				}
			}
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("%s: got %q, err %v; want error containing %q", c.expr, got, err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", c.expr, err)
			}
			if got != c.want {
				t.Fatalf("%s = %s, want %s", c.expr, got, c.want)
			}
		})
	}
}
