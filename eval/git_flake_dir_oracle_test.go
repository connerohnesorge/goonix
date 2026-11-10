package eval_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

// TestGitFlakeDirAndSubmodulesMatchNix keeps dir and submodules=1 through
// parseFlakeRef, and checks that getFlake uses the subdirectory while
// fetchTree with submodules=1 includes the submodule tree.
func TestGitFlakeDirAndSubmodulesMatchNix(t *testing.T) {
	oracle.RequireNix224(t)
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatal("git is required")
	}
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	mod := filepath.Join(root, "mod")
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mod, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, "root.txt"), "root\n")
	write(filepath.Join(repo, "flake.nix"), "{ outputs = {...}: { testValue = \"from-root\"; }; }\n")
	write(filepath.Join(repo, "sub", "nested.txt"), "nested\n")
	write(filepath.Join(repo, "sub", "flake.nix"), "{ outputs = {...}: { testValue = \"from-sub\"; }; }\n")
	write(filepath.Join(mod, "mod.txt"), "modfile\n")
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "protocol.file.allow=always"}, args...)...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args, err, out)
		}
	}
	git(mod, "init", "-q")
	git(mod, "add", ".")
	git(mod, "commit", "-q", "-m", "mod")
	git(repo, "init", "-q")
	git(repo, "add", ".")
	git(repo, "commit", "-q", "-m", "init")
	git(repo, "submodule", "add", "-q", mod, "vendor")
	git(repo, "commit", "-q", "-m", "add sub")

	lay := oracle.MustLayout(t)
	url := "git+file://" + repo
	exprs := []string{
		`let t = builtins.fetchTree "` + url + `"; in "${t.narHash} ${builtins.toJSON t.submodules}"`,
		`let t = builtins.fetchTree "` + url + `?submodules=1"; in "${t.narHash} ${builtins.toJSON t.submodules}"`,
		`let t = builtins.fetchTree "` + url + `?dir=sub"; in t.narHash`,
		`let f = builtins.getFlake "` + url + `"; in f.testValue`,
		`let f = builtins.getFlake "` + url + `?dir=sub"; in f.testValue`,
	}
	for _, expr := range exprs {
		want := lay.EvalRaw(t, expr)
		got := evalPkgsExprString(t, expr)
		if got != want {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
	}
	withSub := evalPkgsExprString(t, `let t = builtins.fetchTree "`+url+`?submodules=1"; in t.narHash`)
	without := evalPkgsExprString(t, `let t = builtins.fetchTree "`+url+`"; in t.narHash`)
	if withSub == without {
		t.Fatal("submodules=1 did not change the tree")
	}
}
