package eval_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestFlakeBuiltinsRequireFeatureLikeNix224(t *testing.T) {
	const wantFlakes = "experimental Nix feature 'flakes' is disabled; add '--extra-experimental-features flakes' to enable it"
	const wantFetch = "experimental Nix feature 'fetch-tree' is disabled; add '--extra-experimental-features fetch-tree' to enable it"
	cases := []struct {
		expr string
		frag string
	}{
		{`builtins.getFlake "path:/tmp"`, wantFlakes},
		{`builtins.parseFlakeRef "path:/tmp"`, wantFlakes},
		{`builtins.flakeRefToString { type = "path"; path = "/tmp"; }`, wantFlakes},
		{`builtins.fetchTree { type = "path"; path = "/tmp"; }`, wantFetch},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			ev := eval.NewEvaluator()
			parsed, err := parser.ParseString(tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ev.EvalWithBuiltins(parsed)
			if err == nil || !strings.Contains(err.Error(), tc.frag) {
				t.Fatalf("go error %v", err)
			}
			if _, err := exec.LookPath("nix-instantiate"); err != nil {
				return
			}
			cmd := exec.Command("nix-instantiate",
				"--option", "experimental-features", "nix-command",
				"--option", "substituters", "",
				"--eval", "--json", "-E", tc.expr)
			cmd.Env = []string{"PATH=" + execPath(), "HOME=/tmp"}
			out, nerr := cmd.CombinedOutput()
			if nerr == nil || !strings.Contains(string(out), "experimental Nix feature") {
				// This Nix build does not gate the builtin (newer than 2.24).
				return
			}
			if !strings.Contains(string(out), tc.frag) {
				t.Fatalf("nix error %q\ngo %v", out, err)
			}
		})
	}

	// With flakes enabled, parseFlakeRef matches Nix (oracle sets the feature explicitly).
	ev := eval.NewEvaluator().WithExperimental("flakes")
	parsed, err := parser.ParseString(`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs")`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.String != "github:NixOS/nixpkgs" {
		t.Fatalf("got %q", fv.String)
	}
}

func execPath() string {
	if p := strings.TrimSpace(os.Getenv("PATH")); p != "" {
		return p
	}
	return "/usr/bin:/bin"
}
