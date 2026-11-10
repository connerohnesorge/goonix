package eval_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestDrvPathStringContextMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	d := `derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }`
	cases := []string{
		`builtins.toJSON (builtins.getContext (` + d + `).drvPath)`,
		`let d = ` + d + `; in builtins.toJSON (builtins.getContext "${d.drvPath}")`,
		`builtins.toJSON (builtins.getContext (builtins.unsafeDiscardOutputDependency (` + d + `).drvPath))`,
		`builtins.toJSON (builtins.getContext (builtins.unsafeDiscardOutputDependency (` + d + `).outPath))`,
	}
	for _, expr := range cases {
		want := lay.EvalRaw(t, expr)
		got := evalPkgsExprString(t, expr)
		if want != got {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}

	// nix-instantiate (not --eval) writes the dependency .drv, which DrvDeep
	// context requires before Nix will hash the outer derivation.
	hashCases := []string{
		`let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = d.drvPath; }`,
		`let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = "${d.drvPath}"; }`,
		`let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; outputs = [ "out" "dev" ]; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = d.drvPath; }`,
		`let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = builtins.unsafeDiscardOutputDependency d.drvPath; }`,
	}
	for _, expr := range hashCases {
		want := nixInstantiateDrv(t, lay, expr)
		got := evalPkgsExprString(t, "("+expr+").drvPath")
		if want != got {
			t.Fatalf("drvPath\nexpr=%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}
	direct := nixInstantiateDrv(t, lay, `let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = d.drvPath; }`)
	interp := nixInstantiateDrv(t, lay, `let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in derivation { name = "e"; system = "x86_64-linux"; builder = "/bin/sh"; x = "${d.drvPath}"; }`)
	if direct != interp {
		t.Fatalf("interpolation changed the drvPath\ndirect=%s\ninterp=%s", direct, interp)
	}

	toFileExpr := `let d = derivation { name = "d"; system = "x86_64-linux"; builder = "/bin/sh"; }; in builtins.toFile "f" d.drvPath`
	_, goErr := evalPkgsExprStringAllowError(t, toFileExpr)
	if goErr == nil {
		t.Fatal("toFile of a drvPath must fail")
	}
	nixErr := nixEvalStderr(t, lay, toFileExpr)
	frag := "files created by builtins.toFile may not reference derivations, but f references ="
	if !strings.Contains(goErr.Error(), frag) || !strings.Contains(nixErr, frag) {
		t.Fatalf("toFile drvPath\ngo=%s\nnix=%s", goErr, nixErr)
	}
}

func nixInstantiateDrv(t *testing.T, lay oracle.Layout, expr string) string {
	t.Helper()
	args := append(lay.InstantiateArgs(), "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = lay.HermeticEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-instantiate: %v\n%s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
