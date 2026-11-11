package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestATermEscapeIllegalNameRejectedLikeNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	for _, attrs := range []string{
		`{ name = "na\"me"; system = "x86_64-linux"; builder = "/bin/sh"; }`,
		`{ name = "na\\me"; system = "x86_64-linux"; builder = "/bin/sh"; }`,
	} {
		if _, err := lay.EvalDerivationDrvPathAllowError(attrs); err == nil {
			t.Fatalf("nix accepted invalid name in %q", attrs)
		}
		if err := goDerivationStrictExpectError(t, attrs); err == nil {
			t.Fatalf("goonix accepted invalid name in %q", attrs)
		}
	}
}

func goDerivationStrictExpectError(t *testing.T, attrs string) error {
	t.Helper()
	src := `builtins.derivationStrict ` + attrs
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(src)
	if err != nil {
		return err
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		return err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return err
	}
	_, err = eval.ForceStrictDrvPath(ev, fv)
	return err
}
