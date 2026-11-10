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
		"builtins.toJSON (builtins.fromTOML ''\na = inf\nb = nan\nc = +inf\nd = -inf\n'')",
		`builtins.typeOf (builtins.fromTOML "a = inf").a`,
		`builtins.typeOf (builtins.fromTOML "a = nan").a`,
		`builtins.typeOf (builtins.fromTOML "a = -inf").a`,
		`builtins.toXML { a = 1; b = "x"; }`,
		`builtins.toXML (derivation { name = "x"; builder = "/bin/sh"; system = "x86_64-linux"; })`,
		`let d = derivation { name = "x"; builder = "/bin/sh"; system = "x86_64-linux"; }; in builtins.toXML (d // { self = d; })`,
		`builtins.readFileType "` + hosts + `"`,
		`builtins.toPath "/tmp"`,
		`builtins.toJSON (builtins.hasContext "")`,
		`builtins.toJSON (builtins.hasContext (builtins.unsafeDiscardOutputDependency ""))`,
		`builtins.toJSON (builtins.break 7)`,
		`builtins.toJSON (builtins.traceVerbose "hi" 1)`,
		`builtins.typeOf builtins.currentTime`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs/nixos-24.05")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs/23.05?dir=lib")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs/nixos-24.05")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs/refs/heads/main")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs/refs/heads/main")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs?ref=foo")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs?ref=foo&dir=lib")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs?ref=foo&dir=lib")`,
		`builtins.toJSON (builtins.parseFlakeRef "github:NixOS/nixpkgs?host=example.com&narHash=sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "github:NixOS/nixpkgs?host=example.com&narHash=sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")`,
		`builtins.toJSON (builtins.parseFlakeRef "gitlab:a/b/v1")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "gitlab:a/b/v1")`,
		`builtins.toJSON (builtins.parseFlakeRef "sourcehut:~edolstra/nix-serve")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "sourcehut:~edolstra/nix-serve/main")`,
		`builtins.toJSON (builtins.parseFlakeRef "sourcehut:~edolstra/nix-serve/refs/heads/main")`,
		`builtins.toJSON (builtins.parseFlakeRef "sourcehut:~edolstra/nix-serve/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")`,
		`builtins.toJSON (builtins.parseFlakeRef "nixpkgs")`,
		`builtins.toJSON (builtins.parseFlakeRef "flake:nixpkgs/nixos-24.05")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "nixpkgs")`,
		`builtins.flakeRefToString { type = "indirect"; id = "nixpkgs"; ref = "nixos-24.05"; }`,
		`builtins.toJSON { outPath = "hello"; other = 1; }`,
		`builtins.toJSON { __toString = self: self.msg; msg = "via"; }`,
		`builtins.toJSON (derivation { name = "x"; builder = "/bin/sh"; system = "x86_64-linux"; })`,
		`builtins.toJSON (builtins.parseFlakeRef "git+https://example.com/foo/bar?ref=main&rev=0123456789abcdef0123456789abcdef01234567")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "git+https://example.com/foo/bar?ref=main&rev=0123456789abcdef0123456789abcdef01234567")`,
		`builtins.toJSON (builtins.parseFlakeRef "https://github.com/NixOS/nixpkgs/archive/master.tar.gz")`,
		`builtins.toJSON (builtins.parseFlakeRef "tarball+https://github.com/NixOS/nixpkgs/archive/master.tar.gz")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "tarball+https://example.com/a.tar.gz")`,
		`builtins.toJSON (builtins.parseFlakeRef "gitlab:NixOS%2Fnixpkgs/nixpkgs")`,
		`builtins.toJSON (builtins.parseFlakeRef "gitlab:group%2Fsub/repo/0123456789abcdef0123456789abcdef01234567")`,
		`builtins.flakeRefToString (builtins.parseFlakeRef "gitlab:NixOS%2Fnixpkgs/nixpkgs")`,
		`builtins.toJSON (builtins.parseFlakeRef "path:/tmp/x")`,
		`builtins.flakeRefToString { type = "path"; path = "/tmp/x"; }`,
	}
	note := filepath.Join(t.TempDir(), "note")
	if err := os.WriteFile(note, []byte("json-path-body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases = append(cases, "builtins.toJSON "+note)
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

func TestNix224ErrorTextMatch(t *testing.T) {
	lay := oracle.MustLayout(t)
	dir := t.TempDir()
	flat := filepath.Join(dir, "f")
	if err := os.WriteFile(flat, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		expr   string
		goHas  string
		nixHas string
	}{
		{
			expr:   `builtins.convertHash { hash = "abcd"; hashAlgo = "sha256"; toHashFormat = "base16"; }`,
			goHas:  "hash 'abcd' has wrong length for hash algorithm 'sha256'",
			nixHas: "has wrong length for hash algorithm",
		},
		{
			expr:   `builtins.fetchurl "file:///no/such/goonix-fetchurl"`,
			goHas:  "unable to download 'file:///no/such/goonix-fetchurl': Couldn't read a file:// file (37)",
			nixHas: "unable to download 'file:///no/such/goonix-fetchurl'",
		},
		{
			expr:   `builtins.fetchTarball "file:///no/such/goonix-fetch.tar.gz"`,
			goHas:  "Failed to open archive (Source threw exception: error: unable to download 'file:///no/such/goonix-fetch.tar.gz': Couldn't read a file:// file (37))",
			nixHas: "file:///no/such/goonix-fetch.tar.gz",
		},
		{
			expr:   `builtins.fetchTree { type = "tarball"; url = "file:///no/such/goonix-fetch.tar.gz"; }`,
			goHas:  "Failed to open archive (Source threw exception: error: unable to download 'file:///no/such/goonix-fetch.tar.gz': Couldn't read a file:// file (37))",
			nixHas: "Failed to open archive",
		},
		{
			expr:   `builtins.path { path = "` + flat + `"; sha256 = "0000000000000000000000000000000000000000000000000000000000000000"; }`,
			goHas:  "store path mismatch in (possibly filtered) path added from '" + flat + "'",
			nixHas: "store path mismatch",
		},
		{
			expr:   `builtins.path { path = "` + flat + `"; recursive = false; sha256 = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="; }`,
			goHas:  "store path mismatch in (possibly filtered) path added from '" + flat + "'",
			nixHas: "store path mismatch",
		},
		{
			expr:   `builtins.readDir "/no/such/goonix-readdir-missing"`,
			goHas:  "reading directory /no/such/goonix-readdir-missing: No such file or directory",
			nixHas: "reading directory /no/such/goonix-readdir-missing: No such file or directory",
		},
		{
			expr:   `builtins.readDir (builtins.filterSource (p: t: true) "` + dir + `")`,
			goHas:  "is not valid",
			nixHas: "is not valid",
		},
		{
			expr:   `builtins.toJSON (x: x)`,
			goHas:  "cannot convert a function to JSON",
			nixHas: "cannot convert a function to JSON",
		},
	}
	ver, _ := exec.Command("nix-instantiate", "--version").CombinedOutput()
	for _, c := range cases {
		_, nerr := lay.EvalExprAllowError(t, c.expr)
		_, gerr := evalPkgsExprStringAllowError(t, c.expr)
		if gerr == nil || !strings.Contains(gerr.Error(), c.goHas) {
			t.Fatalf("%s\ngo=%v", c.expr, gerr)
		}
		if nerr == nil || !strings.Contains(nerr.Error(), c.nixHas) {
			t.Fatalf("%s\nnix=%v", c.expr, nerr)
		}
		if strings.Contains(string(ver), "2.24.9") && !strings.Contains(nerr.Error(), c.goHas) {
			t.Fatalf("2.24.9 mismatch %s\nnix=%v\ngo=%v", c.expr, nerr, gerr)
		}
		if strings.Contains(c.expr, "filterSource") && strings.Contains(string(ver), "2.24.9") {
			msg := gerr.Error()
			i := strings.Index(msg, "path '")
			if i < 0 {
				t.Fatalf("go readDir error %q", msg)
			}
			sentence := msg[i:]
			if !strings.Contains(nerr.Error(), sentence) {
				t.Fatalf("nix does not contain %q\nnix=%v\ngo=%v", sentence, nerr, gerr)
			}
		}
	}
}

func TestUnsafeGetAttrPosInheritAndListToAttrsMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	dir := t.TempDir()
	inh := filepath.Join(dir, "inh.nix")
	if err := os.WriteFile(inh, []byte("let\n  x = 1;\nin {\n  inherit x;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	from := filepath.Join(dir, "from.nix")
	if err := os.WriteFile(from, []byte("let\n  s = { x = 1; };\nin {\n  inherit (s) x;\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	l2a := filepath.Join(dir, "l2a.nix")
	if err := os.WriteFile(l2a, []byte("builtins.listToAttrs [\n  {\n    name = \"a\";\n    value = 1;\n  }\n]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{
		`builtins.toJSON (builtins.unsafeGetAttrPos "x" (import ` + nixPath(inh) + `))`,
		`builtins.toJSON (builtins.unsafeGetAttrPos "x" (import ` + nixPath(from) + `))`,
		`builtins.toJSON (builtins.unsafeGetAttrPos "a" (import ` + nixPath(l2a) + `))`,
	} {
		want := lay.EvalRaw(t, expr)
		got := evalPkgsExprString(t, expr)
		if want != got {
			t.Fatalf("%s\nnix=%s\ngo=%s", expr, want, got)
		}
		if !strings.Contains(want, `"column"`) {
			t.Fatalf("nix returned no position: %s", want)
		}
	}
}

func TestFromTOMLDuplicateAndReadFileTypeMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)
	dup := "builtins.fromTOML ''\na = 1\na = 2\n''"
	_, nerr := lay.EvalExprAllowError(t, dup)
	_, gerr := evalPkgsExprStringAllowError(t, dup)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), "already exists") || !strings.Contains(gerr.Error(), `value ("a") already exists`) {
		t.Fatalf("duplicate key\nnix=%v\ngo=%v", nerr, gerr)
	}
	table := "builtins.fromTOML ''\n[t]\nx = 1\n[t]\ny = 2\n''"
	_, nerr = lay.EvalExprAllowError(t, table)
	_, gerr = evalPkgsExprStringAllowError(t, table)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), `table ("t") already exists`) || !strings.Contains(gerr.Error(), `table ("t") already exists`) {
		t.Fatalf("duplicate table\nnix=%v\ngo=%v", nerr, gerr)
	}
	dotted := "builtins.fromTOML ''\na.b = 1\n[a]\n''"
	_, nerr = lay.EvalExprAllowError(t, dotted)
	_, gerr = evalPkgsExprStringAllowError(t, dotted)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), `table ("a") already exists`) || !strings.Contains(gerr.Error(), `table ("a") already exists`) {
		t.Fatalf("dotted then header\nnix=%v\ngo=%v", nerr, gerr)
	}
	arr := "builtins.fromTOML ''\n[[t]]\n[t]\n''"
	_, nerr = lay.EvalExprAllowError(t, arr)
	_, gerr = evalPkgsExprStringAllowError(t, arr)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), `array of tables ("t") already exists`) || !strings.Contains(gerr.Error(), `array of tables ("t") already exists`) {
		t.Fatalf("array then table\nnix=%v\ngo=%v", nerr, gerr)
	}
	reopen := "builtins.toJSON (builtins.fromTOML ''\n[a.b]\nc = 1\n[a]\n'')"
	if want, got := lay.EvalRaw(t, reopen), evalPkgsExprString(t, reopen); want != got {
		t.Fatalf("header parent reopen\nnix=%s\ngo=%s", want, got)
	}
	missing := `builtins.readFileType "/no/such/goonix-readfiletype"`
	_, nerr = lay.EvalExprAllowError(t, missing)
	_, gerr = evalPkgsExprStringAllowError(t, missing)
	want := "path '/no/such/goonix-readfiletype' does not exist"
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), want) || !strings.Contains(gerr.Error(), want) {
		t.Fatalf("readFileType\nnix=%v\ngo=%v", nerr, gerr)
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
