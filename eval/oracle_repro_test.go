package eval_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestOracleReproducersWithNix(t *testing.T) {
	nix, err := exec.LookPath("nix-instantiate")
	if err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")

	cases := []struct {
		name    string
		file    string
		args    []string
		wantErr bool
		wantSub string
	}{
		{
			name:    "bare_final_update_fails",
			file:    "fixpoint_bare_update.nix",
			wantErr: true,
			wantSub: "infinite recursion",
		},
		{
			name:    "extends_intersect",
			file:    "fixpoint_extends.nix",
			args:    []string{"--arg", "libPath", libPath},
			wantSub: "fetchurl",
		},
		{
			name:    "with_self_callpackage",
			file:    "with_self_callpackage.nix",
			wantSub: "7",
		},
		{
			name: "import_nixpkgs_empty_config",
			file: "import_nixpkgs_empty_config.nix",
			args: []string{"--arg", "nixpkgsPath", nixpkgs},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, "testdata", "oracle", tc.file)
			args := append([]string{"--eval", "-I", "nixpkgs=" + nixpkgs}, tc.args...)
			args = append(args, path)
			cmd := exec.Command(nix, args...)
			out, err := cmd.CombinedOutput()
			s := string(out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got output: %s", s)
				}
				if tc.wantSub != "" && !strings.Contains(s, tc.wantSub) {
					t.Fatalf("stderr/out should contain %q: %s", tc.wantSub, s)
				}
				return
			}
			if err != nil {
				t.Fatalf("nix-instantiate: %v\n%s", err, s)
			}
			if tc.wantSub != "" && !strings.Contains(s, tc.wantSub) {
				t.Fatalf("output should contain %q: %s", tc.wantSub, s)
			}
		})
	}
	t.Logf("oracle pin: Nix %s nixpkgs %s", oracle.NixVersion, oracle.NixpkgsRev)
}
