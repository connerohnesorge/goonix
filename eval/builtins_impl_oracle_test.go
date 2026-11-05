package eval_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestBuiltinImplementationsMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	hosts := "/etc/hosts"
	cases := []string{
		`builtins.toJSON builtins.langVersion`,
		`builtins.toJSON (builtins.isNull null)`,
		`builtins.toJSON (builtins.isNull 1)`,
		`builtins.toJSON (builtins.ceil 1.2)`,
		`builtins.toJSON (builtins.floor 1.2)`,
		`builtins.toJSON (builtins.ceil 3)`,
		`builtins.toJSON (builtins.groupBy (n: builtins.substring 0 1 n) [ "ab" "ac" "b" ])`,
		`builtins.toJSON (builtins.partition (n: n == "a") [ "a" "b" "a" ])`,
		`builtins.convertHash { hash = "sha256:0000000000000000000000000000000000000000000000000000000000000001"; toHashFormat = "nix32"; }`,
		`builtins.hashFile "sha256" "` + hosts + `"`,
		`builtins.hashFile "sha1" "` + hosts + `"`,
		"builtins.toJSON (builtins.fromTOML ''\na = 1\nb = \"x\"\nn = 1.5\nflag = true\n'')",
		`builtins.toXML { a = 1; b = "x"; }`,
		`builtins.readFileType "` + hosts + `"`,
		`builtins.toPath "/tmp"`,
		`builtins.toJSON (builtins.hasContext "")`,
		`builtins.toJSON (builtins.hasContext (builtins.unsafeDiscardOutputDependency ""))`,
		`builtins.toJSON (builtins.break 7)`,
		`builtins.toJSON (builtins.traceVerbose "hi" 1)`,
		`builtins.typeOf builtins.currentTime`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs")`,
		`builtins.toJSON (builtins.parseFlakeRef "path:/tmp/x")`,
		`builtins.flakeRefToString { type = "path"; path = "/tmp/x"; }`,
	}
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if want != got {
				t.Fatalf("nix=%q\ngo=%q", want, got)
			}
		})
	}
}

func TestGetFlakeErrorsMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	for _, expr := range []string{
		`builtins.getFlake "not a flake"`,
		`builtins.parseFlakeRef "???"`,
	} {
		_, nixErr := lay.EvalExprAllowError(t, expr)
		if nixErr == nil {
			t.Fatalf("nix accepted %s", expr)
		}
		_, goErr := evalPkgsExprStringAllowError(t, expr)
		if goErr == nil {
			t.Fatalf("go accepted %s", expr)
		}
		for _, frag := range []string{"flake reference", "is not an absolute path"} {
			if !strings.Contains(nixErr.Error(), frag) || !strings.Contains(goErr.Error(), frag) {
				t.Fatalf("expr %s\nnix=%v\ngo=%v", expr, nixErr, goErr)
			}
		}
	}
	missing := t.TempDir()
	expr := `builtins.getFlake "` + missing + `"`
	_, nixErr := lay.EvalExprAllowError(t, expr)
	_, goErr := evalPkgsExprStringAllowError(t, expr)
	if nixErr == nil || goErr == nil {
		t.Fatalf("expected missing flake to fail nix=%v go=%v", nixErr, goErr)
	}
	want := "does not exist"
	if !strings.Contains(nixErr.Error(), want) || !strings.Contains(goErr.Error(), want) {
		t.Fatalf("nix=%v\ngo=%v", nixErr, goErr)
	}
}

func TestFetchAndPathBuiltinsMatchNix(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	if _, err := exec.LookPath("nix-hash"); err != nil {
		t.Skip("no nix-hash")
	}
	lay := oracle.MustLayout(t)

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "keep"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(src, "drop"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "drop", "gone"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	filterExpr := `builtins.filterSource (path: type: baseNameOf path != "drop") ` + nixPath(src)
	pathExpr := `builtins.path { path = ` + nixPath(src) + `; name = "srcname"; filter = (path: type: baseNameOf path != "drop"); }`
	plainExpr := `builtins.path { path = ` + nixPath(src) + `; name = "srcname"; }`

	repo := t.TempDir()
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "f"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(repo, "add", "f")
	run(repo, "commit", "-q", "-m", "c")
	rev := strings.TrimSpace(string(mustCmd(t, repo, "git", "rev-parse", "HEAD")))

	tarDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tarDir, "f"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(t.TempDir(), "a.tgz")
	if out, err := exec.Command("tar", "-czf", tgz, "-C", tarDir, "f").CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	unpack := t.TempDir()
	if out, err := exec.Command("tar", "-xzf", tgz, "-C", unpack).CombinedOutput(); err != nil {
		t.Fatalf("untar: %v\n%s", err, out)
	}
	sri := strings.TrimSpace(string(mustCmd(t, "", "nix-hash", "--type", "sha256", "--sri", unpack)))
	flat := strings.TrimSpace(string(mustCmd(t, "", "nix-hash", "--type", "sha256", "--flat", filepath.Join(src, "keep"))))

	flake := t.TempDir()
	if err := os.WriteFile(filepath.Join(flake, "flake.nix"), []byte("{ outputs = { self }: { testValue = 42; }; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []string{
		filterExpr,
		pathExpr,
		plainExpr,
		`builtins.fetchurl { url = "file://` + filepath.Join(src, "keep") + `"; name = "keep"; sha256 = "` + flat + `"; }`,
		`builtins.fetchTarball { url = "file://` + tgz + `"; sha256 = "` + sri + `"; name = "tarball-test"; }`,
		`builtins.fetchGit { url = "file://` + repo + `"; rev = "` + rev + `"; name = "goonix-src"; }`,
		`(builtins.getFlake "path:` + flake + `").outPath`,
		`builtins.toString ((builtins.getFlake "path:` + flake + `").outputs.testValue)`,
		`builtins.toJSON (builtins.scopedImport { x = 9; } ` + nixPath(writeFile(t, "x + 1\n")) + `)`,
	}
	for _, expr := range cases {
		t.Run(expr, func(t *testing.T) {
			want := lay.EvalRaw(t, expr)
			got := evalPkgsExprString(t, expr)
			if want != got {
				t.Fatalf("nix=%q\ngo=%q", want, got)
			}
		})
	}
	assertFetchTreeLikeNix(t, lay, `builtins.fetchTree { type = "git"; url = "file://`+repo+`"; rev = "`+rev+`"; shallow = false; }`)
	assertFetchTreeLikeNix(t, lay, `builtins.fetchTree { type = "path"; path = `+nixPath(src)+`; }`)
}

func nixPath(p string) string {
	return `"` + p + `"`
}

func writeFile(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "m.nix")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustCmd(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}
