package builder_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestBuildChainQuotedArgsMatchesNixBuild(t *testing.T) {
	if _, err := exec.LookPath("nix-build"); err != nil {
		t.Skip("no nix-build")
	}
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("no nix-store")
	}

	bottomAttrs := `{ name = "chain-bottom"; system = "x86_64-linux"; builder = "/bin/sh"; args = [ "-c" "echo bottom > $out" ]; }`
	topAttrs := `let bottom = builtins.derivationStrict ` + bottomAttrs + `; in builtins.derivationStrict {
	  name = "chain-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [
	    "-c"
	    "while IFS= read -r line || [ -n \"$line\" ]; do printf \"%s\\n\" \"$line\"; done < ${bottom.out} > $out; printf \"%s\" \"tab\\tquote\" >> $out"
	  ];
	}`
	envAttrs := `{ name = "chain-env"; system = "x86_64-linux"; builder = "/bin/sh"; args = [
	  "-c" "echo env > $out"
	  "\"quoted\""
	  "back\\\\slash"
	  "tab\tchar"
	  "line\nbreak"
	]; }`

	lay := oracle.MustLayout(t)
	wantBottom := lay.EvalStrictOutPath(t, bottomAttrs)
	wantTop := lay.EvalRaw(t, `(`+topAttrs+`).out`)
	wantEnv := lay.EvalStrictOutPath(t, envAttrs)

	goRoot := t.TempDir()
	st, err := local.OpenStore(goRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	goBottom, err := buildStrictEval(t, st, `builtins.derivationStrict `+bottomAttrs)
	if err != nil {
		t.Fatal(err)
	}
	goTop, err := buildStrictEval(t, st, topAttrs)
	if err != nil {
		t.Fatal(err)
	}
	goEnv, err := buildStrictEval(t, st, `builtins.derivationStrict `+envAttrs)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name     string
		want     string
		got      string
		buildNix func(root string) string
	}{
		{"bottom", wantBottom, goBottom, func(r string) string { return oracle.NixBuildDerivation(t, r, bottomAttrs) }},
		{"top", wantTop, goTop, func(r string) string { return oracle.NixBuildExpr(t, r, topBuildExpr(bottomAttrs)) }},
		{"env", wantEnv, goEnv, func(r string) string { return oracle.NixBuildDerivation(t, r, envAttrs) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Fatalf("out path go=%q nix-eval=%q", tc.got, tc.want)
			}
			nixRoot := t.TempDir()
			nixOut := tc.buildNix(nixRoot)
			if nixOut != tc.want {
				t.Fatalf("nix-build out=%q eval=%q", nixOut, tc.want)
			}
			compareNarDump(t, nixRoot, nixOut, goRoot, tc.got)
		})
	}
}

func topBuildExpr(bottomAttrs string) string {
	return `let bottom = derivation ` + bottomAttrs + `; in derivation {
	  name = "chain-top";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [
	    "-c"
	    "while IFS= read -r line || [ -n \"$line\" ]; do printf \"%s\\n\" \"$line\"; done < ${bottom} > $out; printf \"%s\" \"tab\\tquote\" >> $out"
	  ];
	}`
}

func buildStrictEval(t *testing.T, st *local.Store, expr string) (string, error) {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	parsed, err := parser.ParseString(expr)
	if err != nil {
		return "", err
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		return "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		return "", err
	}
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		return "", errNotRegistered
	}
	if err := writeRegisteredClosure(st, ev, drvPath, map[string]bool{}); err != nil {
		return "", err
	}
	outs, err := store.LogicalOutputPaths(gd)
	if err != nil {
		return "", err
	}
	b := &builder.Builder{Store: st, Config: builder.Config{AllowRelocated: true}}
	if err := b.BuildRealisation(drvPath); err != nil {
		return "", err
	}
	return outs["out"], nil
}

func writeRegisteredClosure(st *local.Store, ev *eval.Evaluator, drvPath string, seen map[string]bool) error {
	if seen[drvPath] {
		return nil
	}
	seen[drvPath] = true
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		return errNotRegistered
	}
	for dep := range gd.InputDerivations {
		if err := writeRegisteredClosure(st, ev, dep, seen); err != nil {
			return err
		}
	}
	var buf bytes.Buffer
	if err := gd.WriteDerivation(&buf); err != nil {
		return err
	}
	return st.WriteDerivation(buf.Bytes(), drvPath)
}

var errNotRegistered = evalErrMsg("derivation not registered")

type evalErrMsg string

func (e evalErrMsg) Error() string { return string(e) }

func compareNarDump(t *testing.T, nixStoreRoot, nixOut, goStoreRoot, goOut string) {
	t.Helper()
	goPhys := local.PhysicalPath(goStoreRoot, goOut)
	nixPhys := filepath.Join(nixStoreRoot, "nix", "store", filepath.Base(nixOut))
	goInfo, errGo := os.Stat(goPhys)
	nixInfo, errNix := os.Stat(nixPhys)
	if errGo != nil || errNix != nil {
		t.Fatalf("stat outputs: go=%v nix=%v", errGo, errNix)
	}
	if goInfo.IsDir() != nixInfo.IsDir() {
		t.Fatalf("output kind mismatch go dir=%v nix dir=%v", goInfo.IsDir(), nixInfo.IsDir())
	}
	if !goInfo.IsDir() {
		gb, err := os.ReadFile(goPhys)
		if err != nil {
			t.Fatal(err)
		}
		nb, err := os.ReadFile(nixPhys)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(gb, nb) {
			t.Fatalf("flat output mismatch go=%q nix=%q", gb, nb)
		}
		return
	}
	nixDump, err := exec.Command("nix-store", "--store", oracle.LocalStoreURI(nixStoreRoot), "--dump", nixOut).Output()
	if err != nil {
		t.Fatalf("nix-store --dump nix: %v", err)
	}
	goDump, err := exec.Command("nix-store", "--dump", goPhys).Output()
	if err != nil {
		t.Fatalf("nix-store --dump go: %v", err)
	}
	wantHash, _, err := local.HashNARStream(bytes.NewReader(nixDump))
	if err != nil {
		t.Fatal(err)
	}
	gotHash, _, err := local.HashNARStream(bytes.NewReader(goDump))
	if err != nil {
		t.Fatal(err)
	}
	if gotHash != wantHash {
		t.Fatalf("NAR hash go=%s nix=%s", gotHash, wantHash)
	}
}
