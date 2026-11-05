package store_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store"
)

func TestSanitizeStorePathNameAmpersandInQuery(t *testing.T) {
	raw := "meson.patch?h=mingw-w64-xorgproto&id=7b817efc3144a50e6766817c4ca7242f8ce49307"
	got := store.SanitizeStorePathName(raw)
	want := "meson.patch?h=mingw-w64-xorgproto-id=7b817efc3144a50e6766817c4ca7242f8ce49307"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSanitizeStorePathNameIdentityWhenValid(t *testing.T) {
	name := "xorgproto-2024.1"
	if got := store.SanitizeStorePathName(name); got != name {
		t.Fatalf("got %q want %q", got, name)
	}
}

func TestFetchurlPatchOutPathMatchesNixOracle(t *testing.T) {
	oracle.SkipIfUnavailable(t)
	lay := oracle.MustLayout(t)
	// Nix fetchurl.nix rejects default names containing '&'; nixpkgs sanitizes via lib.strings first.
	wantExpr := `let
  fetchurl = import ` + lay.Nix + `/fetchurl.nix;
  p = fetchurl {
    url = "https://aur.archlinux.org/cgit/aur.git/plain/meson.patch?h=mingw-w64-xorgproto&id=7b817efc3144a50e6766817c4ca7242f8ce49307";
    sha256 = "sha256-Izzz9In53W7CC++k1bLr78iSrmxpFm1cH8qcSpptoUQ=";
    name = "meson.patch?h=mingw-w64-xorgproto-id=7b817efc3144a50e6766817c4ca7242f8ce49307";
  };
in p.outPath`
	want := lay.EvalRaw(t, wantExpr)
	goExpr := `let
  fetchurl = import ` + lay.Nix + `/fetchurl.nix;
  p = fetchurl {
    url = "https://aur.archlinux.org/cgit/aur.git/plain/meson.patch?h=mingw-w64-xorgproto&id=7b817efc3144a50e6766817c4ca7242f8ce49307";
    sha256 = "sha256-Izzz9In53W7CC++k1bLr78iSrmxpFm1cH8qcSpptoUQ=";
    name = "meson.patch?h=mingw-w64-xorgproto-id=7b817efc3144a50e6766817c4ca7242f8ce49307";
  };
in p.outPath`
	got := evalFetchurlOutPathGo(t, goExpr)
	if want != got {
		t.Fatalf("outPath\nnix=%q\ngo=%q", want, got)
	}
}
