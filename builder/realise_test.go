package builder_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

// fakeSubstituter has exactly the paths in have.
type fakeSubstituter struct {
	have  map[string]string
	asked []string
}

func (f *fakeSubstituter) TrySubstitute(st *local.Store, p string) (bool, error) {
	f.asked = append(f.asked, p)
	body, ok := f.have[p]
	if !ok {
		return false, nil
	}
	phys := local.PhysicalPath(st.Root, p)
	if err := os.WriteFile(phys, []byte(body), 0o444); err != nil {
		return false, err
	}
	h, n, err := local.NarHashAndSize(phys)
	if err != nil {
		return false, err
	}
	return true, st.RegisterValidPath(p, h, n, nil)
}

// When the root's outputs can be substituted, Nix never realises the root's
// inputs. The input here fails if it is ever built, and the substituter must
// not be asked for it.
func TestRealiseSubstitutableRootSkipsInputs(t *testing.T) {
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	expr := `let
	  dep = derivation { name = "never-built"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "-c" "exit 1" ]; };
	in derivation { name = "top"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "-c" "echo ${dep} > $out" ]; }`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	parsed, err := parser.ParseString(expr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeRegisteredClosure(st, ev, drvPath, map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		t.Fatal(errNotRegistered)
	}
	outs, err := store.LogicalOutputPaths(gd)
	if err != nil {
		t.Fatal(err)
	}
	sub := &fakeSubstituter{have: map[string]string{outs["out"]: "cached\n"}}
	b := &builder.Builder{Store: st, Config: builder.Config{Substituter: sub}}
	if err := b.BuildRealisation(drvPath); err != nil {
		t.Fatal(err)
	}
	if len(sub.asked) != 1 || sub.asked[0] != outs["out"] {
		t.Fatalf("substituter asked for %v, want only %s", sub.asked, outs["out"])
	}
	body, err := os.ReadFile(local.PhysicalPath(st.Root, outs["out"]))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "cached\n" {
		t.Fatalf("out %q", body)
	}
	entries, err := filepath.Glob(filepath.Join(st.Root, "store", "*-never-built"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("input was built: %v", entries)
	}
}
