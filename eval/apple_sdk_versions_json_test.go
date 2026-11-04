package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestAppleSDKVersionsJSONSelect11(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "nixpkgs", "pkgs", "by-name", "ap", "apple-sdk", "metadata", "versions.json")
	src := `let v = builtins.fromJSON (builtins.readFile "` + path + `"); in v."11".version or "MISSING"`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got == "MISSING" {
		t.Fatalf("key 11 missing in fromJSON attrset")
	}
	if got != "11.3" {
		t.Fatalf("got version %q", got)
	}
}

func TestAppleSDKVersionsDynamicSelect11(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "nixpkgs", "pkgs", "by-name", "ap", "apple-sdk", "metadata", "versions.json")
	src := `let v = builtins.fromJSON (builtins.readFile "` + path + `"); ver = "11"; in v.${ver}.version or "MISSING"`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	got := forceStringOrPath(t, ev, val)
	if got == "MISSING" {
		t.Fatalf("dynamic select ver=11 missing")
	}
}
