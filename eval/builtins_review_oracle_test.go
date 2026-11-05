package eval_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestReviewBuiltinsMatchNix(t *testing.T) {
	lay := oracle.MustLayout(t)

	toml := strings.Join([]string{
		`title = "TOML Example"`,
		`[owner]`,
		`name = "Tom"`,
		`[database]`,
		`ports = [ 8001, 8002 ]`,
		`enabled = true`,
		`[servers.alpha]`,
		`ip = "10.0.0.1"`,
		`point = { x = 1, y = 2 }`,
		`n = 0x10`,
		`o = 0o10`,
		`b = 0b10`,
		`hex = 0xdead_beef`,
		`[[products]]`,
		`name = "Hammer"`,
		`[[products]]`,
		`[[products]]`,
		`name = "Nail"`,
		`sku = 284758393`,
	}, "\n")
	expr := "builtins.toJSON (builtins.fromTOML ''\n" + toml + "\n'')"
	want := lay.EvalRaw(t, expr)
	got := evalPkgsExprString(t, expr)
	if want != got {
		t.Fatalf("fromTOML\nnix=%s\ngo=%s", want, got)
	}

	// Datetimes are gated by parse-toml-timestamps, and Nix prints them as
	// { _type = "timestamp"; value = "<literal>"; }.
	stamp := "odt1 = 1979-05-27T07:32:00Z\nodt4 = 1979-05-27 07:32:00Z\nld1 = 1979-05-27\nlt1 = 07:32:00\n"
	goStamp := evalWithFeatures(t, "builtins.toJSON (builtins.fromTOML ''\n"+stamp+"'')", "flakes", "parse-toml-timestamps")
	nixStamp := nixEvalFeatures(t, "builtins.toJSON (builtins.fromTOML ''\n"+stamp+"'')", "nix-command flakes parse-toml-timestamps")
	if nixStamp != goStamp {
		t.Fatalf("timestamps\nnix=%s\ngo=%s", nixStamp, goStamp)
	}
	_, goDateErr := evalWithFeaturesErr(t, `builtins.fromTOML "t = 1979-05-27T07:32:00Z"`, "flakes")
	if goDateErr == nil || !strings.Contains(goDateErr.Error(), "Dates and times are not supported") {
		t.Fatalf("datetime gate: %v", goDateErr)
	}

	xmlCases := []string{
		`builtins.toXML { n = null; xs = [ 1 "a" ]; f = x: x; }`,
		`builtins.toXML [ 1 null (x: x) { a = true; } ]`,
		`builtins.toXML 1.0`,
		`builtins.toXML 1.5`,
		`builtins.toXML "hi&<>"`,
		`builtins.toXML ({ a, b ? 1, ... }: a)`,
		`builtins.toXML /etc/hosts`,
	}
	for _, c := range xmlCases {
		if w, g := lay.EvalRaw(t, c), evalPkgsExprString(t, c); w != g {
			t.Fatalf("toXML %s\nnix=%q\ngo=%q", c, w, g)
		}
	}

	hashCases := []string{
		`builtins.convertHash { hash = "0000000000000000000000000000000000000000000000000000000000000001"; hashAlgo = "sha256"; toHashFormat = "nix32"; }`,
		`builtins.convertHash { hash = "0000000000000000000000000000000000000000000000000000000000000001"; hashAlgo = "sha256"; toHashFormat = "base64"; }`,
		`builtins.convertHash { hash = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE="; hashAlgo = "sha256"; toHashFormat = "base16"; }`,
		`builtins.convertHash { hash = "0080000000000000000000000000000000000000000000000000"; hashAlgo = "sha256"; toHashFormat = "sri"; }`,
		`builtins.convertHash { hash = "0000000000000000000000000000000000000000000000000000000000000001"; hashAlgo = "sha256"; toHashFormat = "base32"; }`,
	}
	for _, c := range hashCases {
		if w, g := lay.EvalRaw(t, c), evalPkgsExprString(t, c); w != g {
			t.Fatalf("convertHash %s\nnix=%q\ngo=%q", c, w, g)
		}
	}

	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink("/etc/hosts", link); err != nil {
		t.Fatal(err)
	}
	linkExpr := `builtins.readFileType "` + link + `"`
	if w, g := lay.EvalRaw(t, linkExpr), evalPkgsExprString(t, linkExpr); w != g || w != "symlink" {
		t.Fatalf("readFileType symlink nix=%q go=%q", w, g)
	}

	flat := filepath.Join(t.TempDir(), "flat-file")
	if err := os.WriteFile(flat, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	flatExpr := `builtins.path { path = "` + flat + `"; recursive = false; }`
	if w, g := lay.EvalRaw(t, flatExpr), evalPkgsExprString(t, flatExpr); w != g {
		t.Fatalf("path recursive=false\nnix=%q\ngo=%q", w, g)
	}
	dirExpr := `builtins.path { path = "` + filepath.Dir(flat) + `"; recursive = false; }`
	_, nerr := lay.EvalExprAllowError(t, dirExpr)
	_, gerr := evalPkgsExprStringAllowError(t, dirExpr)
	if nerr == nil || gerr == nil || !strings.Contains(gerr.Error(), "Is a directory") {
		t.Fatalf("path dir nix=%v go=%v", nerr, gerr)
	}
	if !strings.Contains(nerr.Error(), "Is a directory") && !strings.Contains(nerr.Error(), "not a regular file") {
		t.Fatalf("path dir nix=%v go=%v", nerr, gerr)
	}

	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "keep"), []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("keep", filepath.Join(src, "via")); err != nil {
		t.Fatal(err)
	}
	filterExpr := `builtins.filterSource (path: type: true) "` + src + `"`
	pathExpr := `builtins.path { path = "` + src + `"; name = "withlink"; }`
	for _, c := range []string{filterExpr, pathExpr} {
		if w, g := lay.EvalRaw(t, c), evalPkgsExprString(t, c); w != g {
			t.Fatalf("symlink nar %s\nnix=%q\ngo=%q", c, w, g)
		}
	}

	fetchExpr := `builtins.fetchurl "file://` + flat + `"`
	if w, g := lay.EvalRaw(t, fetchExpr), evalPkgsExprString(t, fetchExpr); w != g {
		t.Fatalf("fetchurl string\nnix=%q\ngo=%q", w, g)
	}

	fileExpr := `builtins.fetchTree { type = "file"; url = "file://` + flat + `"; }`
	if w, g := lay.EvalRaw(t, fileExpr), evalPkgsExprString(t, fileExpr); w != g {
		t.Fatalf("fetchTree file\nnix=%q\ngo=%q", w, g)
	}
	tarDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tarDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tarDir, "sub", "a"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	tgz := filepath.Join(t.TempDir(), "a.tgz")
	if out, err := exec.Command("tar", "-czf", tgz, "-C", tarDir, "sub").CombinedOutput(); err != nil {
		t.Fatalf("tar: %v\n%s", err, out)
	}
	tarExpr := `builtins.fetchTree { type = "tarball"; url = "file://` + tgz + `"; }`
	if w, g := lay.EvalRaw(t, tarExpr), evalPkgsExprString(t, tarExpr); w != g {
		t.Fatalf("fetchTree tarball\nnix=%q\ngo=%q", w, g)
	}
	strTar := `builtins.fetchTree "file://` + tgz + `"`
	if w, g := lay.EvalRaw(t, strTar), evalPkgsExprString(t, strTar); w != g {
		t.Fatalf("fetchTree string tarball\nnix=%q\ngo=%q", w, g)
	}

	remote := []string{
		`builtins.fetchTree { type = "github"; owner = "NixOS"; repo = "definitely-not-a-real-repo-goonix"; }`,
		`builtins.fetchTree { type = "gitlab"; owner = "x"; repo = "y"; }`,
		`builtins.fetchTree { type = "sourcehut"; owner = "~x"; repo = "y"; }`,
		`builtins.fetchTree { type = "indirect"; id = "not-a-real-flake-id-goonix"; }`,
		`builtins.fetchTree { type = "nope"; }`,
		`builtins.getFlake "github:NixOS/definitely-not-a-real-repo-goonix"`,
	}
	for _, c := range remote {
		_, nerr := lay.EvalExprAllowError(t, c)
		_, gerr := evalPkgsExprStringAllowError(t, c)
		if nerr == nil || gerr == nil {
			t.Fatalf("%s nix=%v go=%v", c, nerr, gerr)
		}
		frag := sharedErrorFragment(nerr.Error())
		if frag == "" || !strings.Contains(gerr.Error(), frag) {
			t.Fatalf("%s\nfragment %q\nnix=%v\ngo=%v", c, frag, nerr, gerr)
		}
		if strings.Contains(c, "getFlake") && strings.Contains(strings.ToLower(gerr.Error()), "clone") {
			t.Fatalf("getFlake started a clone: %v", gerr)
		}
	}

	missingPath := "/no/such/flake-goonix-review"
	missExpr := `builtins.getFlake "` + missingPath + `"`
	_, nerr = lay.EvalExprAllowError(t, missExpr)
	_, gerr = evalPkgsExprStringAllowError(t, missExpr)
	if nerr == nil || gerr == nil || !strings.Contains(gerr.Error(), "does not exist") {
		t.Fatalf("missing path nix=%v go=%v", nerr, gerr)
	}
	if !strings.Contains(nerr.Error(), "does not exist") && !strings.Contains(nerr.Error(), "No such file or directory") {
		t.Fatalf("missing path nix=%v go=%v", nerr, gerr)
	}

	empty := t.TempDir()
	emptyExpr := `builtins.getFlake "` + empty + `"`
	_, nerr = lay.EvalExprAllowError(t, emptyExpr)
	_, gerr = evalPkgsExprStringAllowError(t, emptyExpr)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), "does not exist") || !strings.Contains(gerr.Error(), "does not exist") {
		t.Fatalf("empty flake nix=%v go=%v", nerr, gerr)
	}

	posFile := filepath.Join(t.TempDir(), "pos.nix")
	if err := os.WriteFile(posFile, []byte("{ a = 1; b = 2; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	posExpr := `builtins.toJSON (builtins.unsafeGetAttrPos "a" (import ` + posFile + `))`
	if w, g := lay.EvalRaw(t, posExpr), evalPkgsExprString(t, posExpr); w != g {
		t.Fatalf("unsafeGetAttrPos\nnix=%s\ngo=%s", w, g)
	}

	storeExpr := `builtins.storePath (` + flatExpr + `)`
	if w, g := lay.EvalRaw(t, storeExpr), evalPkgsExprString(t, storeExpr); w != g {
		t.Fatalf("storePath\nnix=%q\ngo=%q", w, g)
	}
	_, nerr = lay.EvalExprAllowError(t, `builtins.storePath "/tmp"`)
	_, gerr = evalPkgsExprStringAllowError(t, `builtins.storePath "/tmp"`)
	if nerr == nil || gerr == nil || !strings.Contains(nerr.Error(), "is not in the Nix store") || !strings.Contains(gerr.Error(), "is not in the Nix store") {
		t.Fatalf("storePath outside nix=%v go=%v", nerr, gerr)
	}
}

func sharedErrorFragment(nixErr string) string {
	for _, frag := range []string{
		"HTTP error 404",
		"cannot find flake",
		"cannot show unsupported input",
		"No such file or directory",
		"does not exist",
		"is not an absolute path",
		"Unrecognized archive format",
	} {
		if strings.Contains(nixErr, frag) {
			return frag
		}
	}
	return ""
}

func evalWithFeatures(t *testing.T, expr string, features ...string) string {
	t.Helper()
	s, err := evalWithFeaturesErr(t, expr, features...)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func evalWithFeaturesErr(t *testing.T, expr string, features ...string) (string, error) {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{}).WithExperimental(features...)
	parsed, err := parser.ParseString(expr)
	if err != nil {
		return "", err
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		return "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	if fv.Kind != eval.VKString {
		return "", errString(fv.Kind)
	}
	return fv.String, nil
}

type errString eval.ValueKind

func (e errString) Error() string { return "want string" }

func nixEvalFeatures(t *testing.T, expr, features string) string {
	t.Helper()
	cmd := exec.Command("nix-instantiate",
		"--option", "experimental-features", features,
		"--option", "substituters", "",
		"--eval", "--json", "-E", expr)
	cmd.Env = oracle.MustLayout(t).HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix: %v", err)
	}
	var decoded string
	if err := json.Unmarshal(out, &decoded); err == nil {
		return decoded
	}
	return strings.TrimSpace(string(out))
}
