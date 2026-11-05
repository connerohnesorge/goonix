package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

// TestFetchTreeGitHubPinnedRevMatchesNix compares narHash and the store path
// of a small pinned public repo. The fields are read individually: toJSON of
// a set that has outPath prints only the path.
func TestFetchTreeGitHubPinnedRevMatchesNix(t *testing.T) {
	oracle.RequireNix224(t)
	const rev = "5edf11c44bc78a0d334f6334cdaf7d60d732daab"
	// Force each field. A set with outPath must not be passed to toJSON.
	field := func(name string) string {
		return `let t = builtins.fetchTree { type = "github"; owner = "edolstra"; repo = "flake-compat"; rev = "` + rev + `"; }; in t.` + name
	}
	lay := oracle.MustLayout(t)
	for _, name := range []string{"outPath", "narHash"} {
		want := lay.EvalRaw(t, field(name))
		got := evalPkgsExprString(t, field(name))
		if got != want {
			t.Fatalf("%s\ngo=%s\nnix=%s", name, got, want)
		}
		if name == "outPath" && !strings.HasPrefix(got, "/nix/store/") {
			t.Fatalf("outPath %s", got)
		}
		if name == "narHash" && !strings.HasPrefix(got, "sha256-") {
			t.Fatalf("narHash %s", got)
		}
	}
}
