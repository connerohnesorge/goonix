package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFloatEdgeValuesMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	exprs := []string{
		`builtins.toJSON (-0.0)`,
		`builtins.toString (-0.0)`,
		`builtins.toJSON (-1.0 * 0.0)`,
		`builtins.toString (-1.0 * 0.0)`,
		`builtins.toJSON ((-0.0) == 0.0)`,
		`builtins.toJSON ((-0.0) < 0.0)`,
		`builtins.toString ((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200))`,
		`builtins.toJSON (2.2250738585072014e-308)`,
	}
	for _, expr := range exprs {
		want := lay.EvalRaw(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}
	for _, expr := range []string{`5.0e-324`, `1.0e-308`} {
		_, goErr := evalPkgsExprStringAllowError(t, expr)
		_, nixErr := lay.EvalExprAllowError(t, expr)
		frag := "invalid float '" + expr + "'"
		if goErr == nil || nixErr == nil || !strings.Contains(goErr.Error(), frag) || !strings.Contains(nixErr.Error(), frag) {
			t.Fatalf("%s\ngo=%v\nnix=%v", expr, goErr, nixErr)
		}
	}
}

func TestToStringReturningIntMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	bad := []string{
		`let s = { __toString = _: 1; }; in "${s}"`,
		`builtins.toJSON { __toString = _: 1; }`,
		`"${1}"`,
	}
	frag := "cannot coerce an integer to a string: 1"
	for _, expr := range bad {
		_, goErr := evalPkgsExprStringAllowError(t, expr)
		_, nixErr := lay.EvalExprAllowError(t, expr)
		if goErr == nil || nixErr == nil || !strings.Contains(goErr.Error(), frag) || !strings.Contains(nixErr.Error(), frag) {
			t.Fatalf("%s\ngo=%v\nnix=%v", expr, goErr, nixErr)
		}
	}
	ok := `builtins.toString { __toString = _: 1; }`
	if got, want := evalPkgsExprString(t, ok), lay.EvalRaw(t, ok); got != want {
		t.Fatalf("toString\ngo=%s\nnix=%s", got, want)
	}
}

func TestToXMLFloatMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	exprs := []string{
		`builtins.toXML { a = 1.0e20; }`,
		`builtins.toXML { a = 1.0e-7; }`,
		`builtins.toXML { a = 100.0; }`,
		`builtins.toXML { a = 1.0e6; }`,
		`builtins.toXML { a = 1234567.0; }`,
		`builtins.toXML { a = 1.2345678901234567; }`,
	}
	for _, expr := range exprs {
		want := lay.EvalRawExact(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s\nnix=%q\ngo=%q", expr, want, got)
		}
	}
}

func TestFromJSONNumberKindMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	exprs := []string{
		`builtins.typeOf (builtins.fromJSON "100.0")`,
		`builtins.typeOf (builtins.fromJSON "100")`,
		`builtins.typeOf (builtins.fromJSON "1e20")`,
		`builtins.typeOf (builtins.fromJSON "1.0")`,
		`builtins.toJSON (builtins.fromJSON "100.0")`,
		`builtins.toJSON (builtins.fromJSON "100")`,
	}
	for _, expr := range exprs {
		want := lay.EvalRaw(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}
}

func TestFloatFormattingMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	exprs := []string{
		`builtins.toJSON [ 1.0e20 100.0 1.0e-7 1.0 0.1 1.5 1.0e-6 1.0e21 1234567.0 1.2345678901234567 1.0e308 (-1.0e20) (-100.0) 1.0e-4 1.0e-5 1.0e15 1.0e6 ]`,
		`builtins.toJSON (1.0e200 * 1.0e200)`,
		`builtins.toJSON ((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200))`,
		`toString 100.0`,
		`toString 1.0e20`,
		`toString 1.0e-7`,
		`toString (1.0e200 * 1.0e200)`,
		`toString (0.0 - (1.0e200 * 1.0e200))`,
		`toString ((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200))`,
		`builtins.toJSON (((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200)) == ((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200)))`,
	}
	for _, expr := range exprs {
		want := lay.EvalRaw(t, expr)
		got, err := goEvalString(t, expr)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}

	plain := `{ name = "float-env"; system = "x86_64-linux"; builder = "/bin/sh"; a = 100.0; b = 1.0e20; c = 1.0e-7; }`
	structured := `{ __structuredAttrs = true; name = "float-json"; system = "x86_64-linux"; builder = "/bin/sh"; a = 100.0; b = 1.0e20; c = 1.0e-7; }`
	for _, attrs := range []string{plain, structured} {
		want := lay.EvalStrictDrvPath(t, attrs)
		got, _ := goDerivationStrictOracle(t, "", attrs)
		if got != want {
			t.Fatalf("drvPath\nattrs=%s\nnix=%s\ngo=%s", attrs, want, got)
		}
	}
}

func TestNaNIsNotEqualToItself(t *testing.T) {
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(`((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200)) == ((1.0e200 * 1.0e200) - (1.0e200 * 1.0e200))`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKBool || fv.Bool {
		t.Fatalf("NaN compared equal: %#v", fv)
	}
}
