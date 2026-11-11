package eval_test

import (
	"fmt"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/internal/testutil"
)

func TestURILiteralWithQueryMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	expr := `https://example.com/pkg?rev=abc&format=tar`
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("uri literal\nnix=%q\ngo=%q", want, got)
	}
}

func TestURIInStringWithQueryMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	expr := `"https://example.com/bar?x=1&y=2"`
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("string uri\nnix=%q\ngo=%q", want, got)
	}
}

func TestBaseNameOfURLWithQueryMatchesNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	url := "https://aur.archlinux.org/cgit/aur.git/plain/meson.patch?h=mingw-w64-xorgproto&id=7b817efc3144a50e6766817c4ca7242f8ce49307"
	expr := fmt.Sprintf(`builtins.baseNameOf %q`, url)
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("baseNameOf\nnix=%q\ngo=%q", want, got)
	}
}


func TestXorgLibX11DrvPathMatchesNixOracle(t *testing.T) {
	assertPkgDrvPathMatchesNix(t, `pkgs.xorg.libX11.drvPath`)
}

func TestPkgsDrvPathSmokeMatchesNixOracle(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		nixOnly bool
	}{
		{name: "python3", expr: `pkgs.python3.drvPath`},
		{name: "openssl", expr: `pkgs.openssl.drvPath`},
		{name: "xorg_libXext", expr: `pkgs.xorg.libXext.drvPath`},
		{name: "xorg_xcbproto", expr: `pkgs.xorg.xcbproto.drvPath`},
		{name: "git", expr: `pkgs.git.out.drvPath`},
	}
	nixpkgs := testutil.NixpkgsPath(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			full := fmt.Sprintf(`let pkgs = %s; in %s`, pkgsImportExpr(nixpkgs), tc.expr)
			lay := oracle.MustLayout(t)
			want := lay.EvalRaw(t, full)
			if tc.nixOnly {
				if want == "" {
					t.Fatal("empty nix drvPath")
				}
				got, err := tryEvalDrvPathString(t, full)
				if err != nil {
					t.Skipf("go: %v (nix oracle %q)", err, want)
				}
				if got != want {
					t.Fatalf("drvPath\nnix=%q\ngo=%q", want, got)
				}
				return
			}
			assertExprDrvPathMatchesNix(t, full)
		})
	}
}

func assertPkgDrvPathMatchesNix(t *testing.T, tail string) {
	t.Helper()
	nixpkgs := testutil.NixpkgsPath(t)
	assertExprDrvPathMatchesNix(t, fmt.Sprintf(`let pkgs = %s; in %s`, pkgsImportExpr(nixpkgs), tail))
}

func assertExprDrvPathMatchesNix(t *testing.T, expr string) {
	t.Helper()
	lay := oracle.MustLayout(t)
	want := lay.EvalRaw(t, expr)
	got, err := tryEvalDrvPathString(t, expr)
	if err != nil {
		t.Fatal(err)
	}
	if want != got {
		t.Fatalf("drvPath\nnix=%q\ngo=%q", want, got)
	}
}

func tryEvalDrvPathString(t *testing.T, expr string) (string, error) {
	t.Helper()
	return evalPkgsExprStringAllowError(t, expr)
}
