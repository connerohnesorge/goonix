package eval_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestPathPlusOracleMatchesNix(t *testing.T) {
	root := modRoot(t)
	dir := filepath.Join(root, "eval", "testdata")
	aDir := filepath.Join(dir, "path_plus_a")
	if err := os.MkdirAll(aDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yFile := filepath.Join(dir, "path_plus_y.txt")
	if err := os.WriteFile(yFile, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(aDir)
		_ = os.Remove(yFile)
	})

	lay := oracle.MustLayout(t)
	probe := filepath.Join(dir, "path_plus_oracle.nix")
	wantRaw := lay.EvalFileRaw(t, probe)
	gotRaw := evalGoonixFileRaw(t, probe)
	if gotRaw != wantRaw {
		t.Fatalf("path plus oracle\nnix=%s\ngo=%s", wantRaw, gotRaw)
	}
	var want, got struct {
		TypeOfPathSuffix     string `json:"typeOfPathSuffix"`
		StringPlusPathType   string `json:"stringPlusPathType"`
		StringPlusPathCtxLen int    `json:"stringPlusPathCtxLen"`
	}
	if err := json.Unmarshal([]byte(wantRaw), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(gotRaw), &got); err != nil {
		t.Fatal(err)
	}
	if want.TypeOfPathSuffix != "path" || got.TypeOfPathSuffix != "path" {
		t.Fatalf("typeOf (a + \"/b\") want path nix=%q go=%q", want.TypeOfPathSuffix, got.TypeOfPathSuffix)
	}
	if want.StringPlusPathType != "string" || got.StringPlusPathType != "string" {
		t.Fatalf("typeOf (\"pre-\" + p) want string nix=%q go=%q", want.StringPlusPathType, got.StringPlusPathType)
	}
	if want.StringPlusPathCtxLen < 1 || got.StringPlusPathCtxLen < 1 {
		t.Fatalf("getContext (\"pre-\" + p) want non-empty nix=%d go=%d", want.StringPlusPathCtxLen, got.StringPlusPathCtxLen)
	}
}

func TestPathPlusStoreStringErrorsLikeNix(t *testing.T) {
	root := modRoot(t)
	dir := filepath.Join(root, "eval", "testdata")
	if err := os.MkdirAll(filepath.Join(dir, "path_plus_a"), 0o755); err != nil {
		t.Fatal(err)
	}
	lay := oracle.MustLayout(t)
	expr := `let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; a = ./eval/testdata/path_plus_a; in a + pkgs.zlib.outPath`
	nixOut, nixErr := lay.EvalExprAllowError(t, expr)
	if nixErr == nil {
		t.Fatalf("nix expected error, got %q", nixOut)
	}
	if !strings.Contains(nixErr.Error(), "cannot be appended to a path") {
		t.Fatalf("nix error %q", nixErr)
	}

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(filepath.Join(dir, "path_plus_oracle.nix"))
	parsed, err := parser.ParseString(`let pkgs = import <nixpkgs> { config = {}; overlays = []; system = "x86_64-linux"; }; a = ./eval/testdata/path_plus_a; in a + pkgs.zlib.outPath`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(parsed)
	if err == nil {
		t.Fatal("go expected error for path + derivation string")
	}
	if !strings.Contains(err.Error(), "cannot be appended to a path") {
		t.Fatalf("go error %q", err)
	}
}

func TestPathGetContextErrorsLikeNix(t *testing.T) {
	root := modRoot(t)
	dir := filepath.Join(root, "eval", "testdata")
	if err := os.MkdirAll(filepath.Join(dir, "path_plus_a"), 0o755); err != nil {
		t.Fatal(err)
	}
	lay := oracle.MustLayout(t)
	expr := `let a = ./eval/testdata/path_plus_a; in builtins.getContext (a + "/b")`
	_, nixErr := lay.EvalExprAllowError(t, expr)
	if nixErr == nil {
		t.Fatal("nix expected getContext on path to fail")
	}
	if !strings.Contains(nixErr.Error(), "expected a string but found a path") {
		t.Fatalf("nix error %q", nixErr)
	}

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(filepath.Join(dir, "path_plus_oracle.nix"))
	parsed, err := parser.ParseString(`let a = ./eval/testdata/path_plus_a; in builtins.getContext (a + "/b")`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(parsed)
	if err == nil {
		t.Fatal("go expected getContext on path to fail")
	}
	if !strings.Contains(err.Error(), "expected a string but found a path") {
		t.Fatalf("go error %q", err)
	}
}
