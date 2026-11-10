package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

// TestFetchTreeAttrsetRefMatchesNix checks that the attrset form of fetchTree
// resolves ref the way the string form does. A missing ref used to fetch the
// default branch (flake-compat 5edf11c instead of tag v1.0.0 at 4f910c9).
func TestFetchTreeAttrsetRefMatchesNix(t *testing.T) {
	oracle.RequireNix224(t)
	lay := oracle.MustLayout(t)
	refs := []string{
		`{ type = "github"; owner = "edolstra"; repo = "flake-compat"; ref = "v1.0.0"; }`,
		`{ type = "gitlab"; owner = "gitlab-org"; repo = "gitlab-test"; ref = "v1.0.0"; }`,
		`{ type = "sourcehut"; owner = "~sircmpwn"; repo = "scdoc"; ref = "1.11.2"; }`,
	}
	for _, ref := range refs {
		for _, name := range []string{"rev", "shortRev", "narHash"} {
			expr := `let t = builtins.fetchTree ` + ref + `; in t.` + name
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if got != want {
				t.Fatalf("%s %s\ngo=%s\nnix=%s", ref, name, got, want)
			}
		}
	}
	pinned := `let t = builtins.fetchTree { type = "github"; owner = "edolstra"; repo = "flake-compat"; ref = "v1.0.0"; }; in t.shortRev`
	if got := evalPkgsExprString(t, pinned); got != "4f910c9" {
		t.Fatalf("flake-compat v1.0.0 shortRev %s", got)
	}
}
