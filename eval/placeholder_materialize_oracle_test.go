package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store"
)

func TestPlaceholderMaterializesInDerivationStrictEnv(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	lay := oracle.MustLayout(t)
	expr := `(builtins.derivationStrict {
  name = "goonix-placeholder-oracle";
  system = "x86_64-linux";
  builder = "/bin/sh";
  outputs = [ "out" "devdoc" ];
  configureFlags = "-Dprefix=" + (builtins.placeholder "out") + " -Ddoc=" + (builtins.placeholder "devdoc");
}).drvPath`
	nixPath := lay.EvalRaw(t, expr)
	goPath, ev := evalProbeExpr(t, expr)
	gd, ok := ev.RegisteredDerivation(goPath)
	if !ok {
		t.Fatal("not registered")
	}
	nixATerm, err := os.ReadFile(nixPath)
	if err != nil {
		t.Skip("nix placeholder drv not on disk")
	}
	goATerm, err := eval.SerializeGoNixDerivation(gd)
	if err != nil {
		t.Fatal(err)
	}
	if string(nixATerm) != string(goATerm) {
		t.Logf("drvPath nix=%s go=%s", nixPath, goPath)
	}
	cfg := gd.Env["configureFlags"]
	if strings.Contains(cfg, "unknown-tee") {
		t.Fatalf("configureFlags still has placeholder text: %q", cfg)
	}
	outRef := "/" + strings.Split(strings.TrimPrefix(gd.Env["out"], store.DefaultStoreDir+"/"), "-")[0]
	if !strings.Contains(cfg, outRef) {
		t.Fatalf("configureFlags=%q want ref %q out=%q", cfg, outRef, gd.Env["out"])
	}
}

func evalProbeExpr(t *testing.T, expr string) (string, *eval.Evaluator) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe.nix")
	if err := os.WriteFile(probe, []byte(expr), 0o644); err != nil {
		t.Fatal(err)
	}
	return evalProbeDrvPath(t, probe)
}
