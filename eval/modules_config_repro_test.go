package eval_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestEvalModulesTopLevelConfigNix(t *testing.T) {
	root := modRoot(t)
	libPath := filepath.Join(root, "testdata", "nixpkgs", "lib", "default.nix")
	configPath := filepath.Join(root, "testdata", "nixpkgs", "pkgs", "top-level", "config.nix")
	src := `
let
  lib = import "` + libPath + `";
  config0 = { };
  config1 = config0;
  configEval = lib.evalModules {
    modules = [
      (import "` + configPath + `")
      ({ config = config1; })
    ];
    class = "nixpkgsConfig";
  };
in configEval.config
`
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
}
