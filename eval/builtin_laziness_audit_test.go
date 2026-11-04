package eval_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

const forcedMsg = "forced"

// lazinessCase: evaluating expr must succeed; expr is written so a strict builtin would throw forcedMsg.
type lazinessCase struct {
	name string
	expr string
}

func evalExprString(t *testing.T, src string) *eval.Value {
	t.Helper()
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatalf("force: %v", err)
	}
	return fv
}

func nixTryEvalSuccess(t *testing.T, expr string) (bool, bool) {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		return false, false
	}
	wrapped := "(builtins.tryEval (" + expr + ")).success"
	cmd := exec.Command("nix-instantiate", "--eval", "-E", wrapped)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("nix oracle failed: %v\n%s", err, out)
		return false, false
	}
	return strings.TrimSpace(string(out)) == "true", true
}

func TestBuiltinLazinessAudit(t *testing.T) {
	throwVal := `throw "` + forcedMsg + `"`

	cases := []lazinessCase{
		// --- attrset builtins ---
		{name: "mapAttrs unused value", expr: `builtins.mapAttrs (n: v: 1) { a = 1; b = ` + throwVal + `; }`},
		{name: "mapAttrs used only a", expr: `(builtins.mapAttrs (n: v: v + 1) { a = 1; b = ` + throwVal + `; }).a`},
		{name: "zipAttrsWith one key", expr: `(builtins.zipAttrsWith (n: vals: builtins.head vals) [ { a = 1; b = ` + throwVal + `; } { a = 2; } ]).a`},
		{name: "removeAttrs keeps lazy attrs", expr: `(builtins.removeAttrs { a = 1; b = ` + throwVal + `; } [ "c" ]).a`},
		{name: "intersectAttrs", expr: `(builtins.intersectAttrs { a = 1; b = ` + throwVal + `; } { a = 2; c = 3; }).a`},
		{name: "getAttr", expr: `builtins.getAttr { a = 1; b = ` + throwVal + `; } "a"`},
		{name: "hasAttr", expr: `builtins.hasAttr "a" { a = 1; b = ` + throwVal + `; }`},
		{name: "attrNames", expr: `builtins.length (builtins.attrNames { a = 1; b = ` + throwVal + `; })`},
		{name: "attrValues head", expr: `builtins.head (builtins.attrValues { a = 1; b = ` + throwVal + `; })`},
		{name: "catAttrs", expr: `builtins.head (builtins.catAttrs "a" [ { a = 1; z = ` + throwVal + `; } ])`},
		{name: "listToAttrs", expr: `(builtins.listToAttrs [ { name = "a"; value = 1; } { name = "b"; value = ` + throwVal + `; } ]).a`},

		// --- list builtins ---
		{name: "map head", expr: `builtins.head (builtins.map (x: x) [ 1 ( ` + throwVal + ` ) ])`},
		{name: "filter copy", expr: `builtins.length (builtins.filter (x: true) [ { z = ` + throwVal + `; } ])`},
		{name: "genList head", expr: `builtins.head (builtins.genList (i: i) 2)`},
		{name: "length", expr: `builtins.length [ 1 ( ` + throwVal + ` ) ]`},
		{name: "elemAt", expr: `builtins.elemAt [ 1 ( ` + throwVal + ` ) ] 0`},
		{name: "head", expr: `builtins.head [ 1 ( ` + throwVal + ` ) ]`},
		{name: "tail length", expr: `builtins.length (builtins.tail [ 1 ( ` + throwVal + ` ) ])`},
		{name: "concatLists length", expr: `builtins.length (builtins.concatLists [ [ 1 ] [ ( ` + throwVal + ` ) ] ])`},
		{name: "foldl' short via any", expr: `builtins.any (x: x == 1) [ 1 ( ` + throwVal + ` ) ]`},

		// --- functionArgs / addErrorContext ---
		{name: "functionArgs", expr: `builtins.length (builtins.attrNames (builtins.functionArgs (a: b: a)))`},
		{name: "addErrorContext", expr: `builtins.addErrorContext "ctx" (if false then ` + throwVal + ` else 1)`},

		// --- operators / forms ---
		{name: "update unused rhs", expr: `({ a = 1; b = ` + throwVal + `; } // { c = 3; }).a`},
		{name: "update unused lhs value", expr: `({ a = 1; } // { b = ` + throwVal + `; }).a`},
		{name: "hasAttr ?", expr: `{ a = 1; b = ` + throwVal + `; } ? a`},
		{name: "selectOr", expr: `({ a = 1; b = ` + throwVal + `; }.c or 7)`},
		{name: "with", expr: `with { a = 1; z = ` + throwVal + `; }; a`},
		{name: "inherit", expr: `let a = 1; z = ` + throwVal + `; in { inherit a; }`},
		{name: "let unused", expr: `let a = 1; u = ` + throwVal + `; in a`},
		{name: "rec forward", expr: `rec { a = 1; b = a; u = ` + throwVal + `; }.a`},
		{name: "formals unused", expr: `let f = { a, b ? ` + throwVal + ` }: a; in f { a = 1; }`},
		{name: "list append unused", expr: `[ 1 ] ++ (if false then [ ( ` + throwVal + ` ) ] else [ ])`},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ev := eval.NewEvaluator()
			expr, err := parser.ParseString(tc.expr)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			val, err := ev.EvalWithBuiltins(expr)
			if err != nil {
				if ok, ran := nixTryEvalSuccess(t, tc.expr); ran && ok {
					t.Fatalf("goonix strict but nix lazy: %v", err)
				}
				t.Fatalf("eval: %v", err)
			}
			fv, err := ev.ForceValue(val)
			if err != nil {
				if ok, ran := nixTryEvalSuccess(t, tc.expr); ran && ok {
					t.Fatalf("goonix strict but nix lazy: %v", err)
				}
				t.Fatalf("force: %v", err)
			}
			_ = fv
			if ok, ran := nixTryEvalSuccess(t, tc.expr); ran && !ok {
				t.Logf("note: nix tryEval failed but goonix succeeded for %q (goonix may be lazier or test expr differs)", tc.name)
			}
		})
	}
}

func TestLengthLazyDoesNotForceElements(t *testing.T) {
	evalExprString(t, `builtins.length [ 1 ( throw "`+forcedMsg+`" ) ]`)
}

func TestSeqDeepSeqMustForce(t *testing.T) {
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(`builtins.seq (throw "` + forcedMsg + `") 1`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err == nil || !strings.Contains(err.Error(), forcedMsg) {
		t.Fatalf("seq should force first arg during eval, got err=%v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
