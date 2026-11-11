package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

func TestPlaceholderHashMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	wantOut := lay.EvalStdinRaw(t, `builtins.placeholder "out"`)
	if got := store.OutputPlaceholder("out"); got != wantOut {
		t.Fatalf("out: got %q nix %q", got, wantOut)
	}
	wantDev := lay.EvalStdinRaw(t, `builtins.placeholder "dev"`)
	if got := store.OutputPlaceholder("dev"); got != wantDev {
		t.Fatalf("dev: got %q nix %q", got, wantDev)
	}
}

func TestPlaceholderSameNameEqualAndEmptyContext(t *testing.T) {
	lay := oracle.MustLayout(t)
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	for _, tc := range []struct {
		name string
		expr string
		want string
	}{
		{
			name: "repeat",
			expr: `if (builtins.placeholder "out") == (builtins.placeholder "out") then "yes" else "no"`,
			want: "yes",
		},
		{
			name: "ctx",
			expr: `builtins.toString (builtins.length (builtins.attrNames (builtins.getContext (builtins.placeholder "out"))))`,
			want: "0",
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			want := lay.EvalStdinRaw(t, tc.expr)
			expr, err := parser.ParseString(tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			val, err := ev.EvalWithBuiltins(expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := forceStringOrPathEvalTest(t, ev, val)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("go=%q nix=%q", got, want)
			}
		})
	}
}
