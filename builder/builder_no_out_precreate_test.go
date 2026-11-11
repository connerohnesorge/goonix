package builder_test

import (
	"bytes"
	"os"
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

func userMountNSAvailable() bool {
	return exec.Command("unshare", "-rmp", "--fork", "--propagation", "private", "true").Run() == nil
}

func TestBuildRejectsRelocatedStoreWithoutAllowRelocated(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	// A non-default logical prefix is rejected whether or not a user namespace is available.
	t.Run("logical prefix", func(t *testing.T) {
		st, err := local.OpenStoreWithConfig(t.TempDir(), store.Config{StoreDir: "/gnu/store"})
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		err = realiseAttrs(t, st, `{
		  name = "reloc-prefix";
		  system = "x86_64-linux";
		  builder = "/bin/sh";
		  args = [ "-c" "printf %s hi > $out" ];
		}`, false)
		if err == nil {
			t.Fatal("expected non-/nix/store logical prefix to fail without AllowRelocated")
		}
		if !strings.Contains(err.Error(), "AllowRelocated") {
			t.Fatalf("error %q", err)
		}
	})
	t.Run("mount unavailable", func(t *testing.T) {
		if userMountNSAvailable() {
			return
		}
		st, err := local.OpenStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		err = realiseAttrs(t, st, `{
		  name = "reloc-denied";
		  system = "x86_64-linux";
		  builder = "/bin/sh";
		  args = [ "-c" "printf %s hi > $out" ];
		}`, false)
		if err == nil {
			t.Fatal("expected build to fail without AllowRelocated when the mount namespace is unavailable")
		}
		if !strings.Contains(err.Error(), "AllowRelocated") {
			t.Fatalf("error %q", err)
		}
	})
}

func TestBuildWithoutAllowRelocatedUsesLogicalStore(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if !userMountNSAvailable() {
		t.Skip("user mount namespace unavailable")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	attrs := `{
	  name = "logical-out";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "printf %s \"$out\" > \"$out\"" ];
	}`
	if err := realiseAttrs(t, st, attrs, false); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(st.Root + "/store")
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	var name string
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".drv") {
			continue
		}
		name = ent.Name()
		body, err = os.ReadFile(local.PhysicalPath(st.Root, "/nix/store/"+name))
		if err != nil {
			t.Fatal(err)
		}
	}
	if name == "" {
		t.Fatal("no output in store")
	}
	want := "/nix/store/" + name
	if string(body) != want {
		t.Fatalf("builder saw %q, want logical %q", body, want)
	}

	// A builder that exits nonzero must not be retried with physical paths.
	failAttrs := `{
	  name = "logical-fail";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "exit 1" ];
	}`
	if err := realiseAttrs(t, st, failAttrs, true); err == nil {
		t.Fatal("failing builder succeeded after a physical-path retry")
	}
}

func realiseAttrs(t *testing.T, st *local.Store, attrs string, allowRelocated bool) error {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(`builtins.derivationStrict ` + attrs)
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
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		return err
	}
	gd, ok := ev.RegisteredDerivation(drvPath)
	if !ok {
		return errNotRegistered
	}
	var buf bytes.Buffer
	if err := gd.WriteDerivation(&buf); err != nil {
		return err
	}
	if err := st.WriteDerivation(buf.Bytes(), drvPath); err != nil {
		return err
	}
	b := &builder.Builder{Store: st, Config: builder.Config{AllowRelocated: allowRelocated}}
	return b.BuildRealisation(drvPath)
}

func TestBuildWithoutMkdirOutFailsLikeNix(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	attrs := `{
	  name = "no-mkdir-out";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "echo x > $out/file" ];
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
