package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

// TestFetchTreeSourceHutPinnedMatchesNix fetches a small public repo the way
// Nix 2.24.9 does: a fixed commit, an annotated tag, and HEAD. HEAD's commit
// moves, so that case is compared live rather than pinned to a hash.
func TestFetchTreeSourceHutPinnedMatchesNix(t *testing.T) {
	oracle.RequireNix224(t)
	lay := oracle.MustLayout(t)
	refs := []string{
		"sourcehut:~sircmpwn/scdoc/24be30ef17d5ca0162f11b35258c8fa2413d556a",
		"sourcehut:~sircmpwn/scdoc/1.11.2",
		"sourcehut:~sircmpwn/scdoc",
	}
	for _, ref := range refs {
		for _, name := range []string{"outPath", "narHash", "rev", "shortRev"} {
			expr := `let t = builtins.fetchTree "` + ref + `"; in t.` + name
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if got != want {
				t.Fatalf("%s %s\ngo=%s\nnix=%s", ref, name, got, want)
			}
			if name == "rev" && len(got) != 40 {
				t.Fatalf("%s rev %q", ref, got)
			}
			if name == "shortRev" && len(got) != 7 {
				t.Fatalf("%s shortRev %q", ref, got)
			}
		}
	}
	missing := `builtins.fetchTree "sourcehut:~sircmpwn/scdoc/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"`
	_, goErr := evalPkgsExprStringAllowError(t, missing)
	if goErr == nil || !strings.Contains(goErr.Error(), "HTTP error 404") {
		t.Fatalf("missing rev: %v", goErr)
	}
}
