package eval_test

import (
	"fmt"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/internal/testutil"
)

func TestBuiltinMatchSplitSanitizeDerivationNameOracle(t *testing.T) {
	lay := oracle.MustLayout(t)
	libNix := testutil.NixpkgsPath(t) + "/lib/default.nix"
	patchName := "meson.patch?h=mingw-w64-xorgproto&id=7b817efc3144a50e6766817c4ca7242f8ce49307"

	cases := []struct {
		name string
		expr string
	}{
		{
			name: "sanitizeDerivationName patch URL",
			expr: fmt.Sprintf(`let lib = import %q; in lib.strings.sanitizeDerivationName %q`, libNix, patchName),
		},
		{
			name: "split illegal chars",
			expr: fmt.Sprintf(`builtins.concatStringsSep "," (map (x: if builtins.isList x then "-" else x) (builtins.split "[^[:alnum:]+._?=-]+" %q))`, patchName),
		},
		{
			name: "match okRegex fail",
			expr: fmt.Sprintf(`builtins.toString (builtins.match "[[:alnum:]+_?=-][[:alnum:]+._?=-]*" %q == null)`, patchName),
		},
		{
			name: "split newline empty segments",
			expr: `builtins.toString (builtins.length (builtins.filter builtins.isString (builtins.split "\n" "a\nb\n\nc")))`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := lay.EvalRaw(t, tc.expr)
			got := evalPkgsExprString(t, tc.expr)
			if want != got {
				t.Fatalf("oracle mismatch\nnix=%q\ngo=%q", want, got)
			}
		})
	}
}

func TestDerivationInvalidStorePathNameRejectedLikeNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	attrs := `{ name = "a&b"; builder = "x"; system = "x86_64-linux"; }`
	if _, err := lay.EvalDerivationDrvPathAllowError(attrs); err == nil {
		t.Fatal("nix accepted a&b name")
	}
	if err := goDerivationStrictExpectError(t, attrs); err == nil {
		t.Fatal("go accepted a&b name")
	}
}
