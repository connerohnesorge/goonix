package builder_test

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestBuildRejectsRelocatedStoreWithoutAllowRelocated(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if err := exec.Command("unshare", "-m", "true").Run(); err == nil {
		t.Skip("mount namespace available; logical /nix/store bind works without AllowRelocated")
	}
	attrs := `{
	  name = "reloc-denied";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "/bin/mkdir -p $out" ];
	}`
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(`builtins.derivationStrict ` + attrs)
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
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		t.Fatal("derivation not registered")
	}
	var buf bytes.Buffer
	if err := gd.WriteDerivation(&buf); err != nil {
		t.Fatal(err)
	}
	if err := st.WriteDerivation(buf.Bytes(), drvPath); err != nil {
		t.Fatal(err)
	}
	b := &builder.Builder{Store: st}
	err = b.BuildRealisation(drvPath)
	if err == nil {
		t.Fatal("expected relocated store to fail without AllowRelocated")
	}
	if !strings.Contains(err.Error(), "AllowRelocated") {
		t.Fatalf("error %q", err)
	}
}

func TestBuildWithoutMkdirOutSubpathFailsLikeNix(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	attrs := `{
	  name = "no-mkdir-out";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "echo x > $out/sub/file" ];
	}`
	lay := oracle.MustLayout(t)
	if _, err := lay.EvalDerivationDrvPathAllowError(attrs); err == nil {
		// Nix may still instantiate; build must fail.
	}
	// Build with Nix in temp store — must fail without mkdir $out.
	nixStore := t.TempDir()
	nixCmd := exec.Command("nix-build", append([]string{
		"--store", oracle.LocalStoreURI(nixStore),
	}, append(oracle.UnprivilegedBuildArgs(), "-E", "derivation "+attrs)...)...)
	nixCmd.Env = oracle.BuildCmdEnv(nixStore)
	if err := nixCmd.Run(); err == nil {
		t.Fatal("nix-build succeeded without mkdir $out (expected failure)")
	}

	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(`builtins.derivationStrict ` + attrs)
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
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	av, _ := parser.ParseString(attrs)
	attrsV, _ := ev.EvalWithBuiltins(av)
	attrsV, _ = ev.ForceValue(attrsV)
	drv, _ := ev.AttrsToDerivation(attrsV)
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	_ = eval.PrepareDerivationPaths(ev, cfg, drv)
	var buf bytes.Buffer
	aterm, _ := eval.SerializeDerivation(cfg, drv)
	buf.Write(aterm)
	_ = st.WriteDerivation(buf.Bytes(), drvPath)

	b := &builder.Builder{Store: st, Config: builder.Config{AllowRelocated: true}}
	err = b.BuildRealisation(drvPath)
	if err == nil {
		t.Fatal("goonix build succeeded without mkdir $out (expected failure)")
	}
}
