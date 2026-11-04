package ladder_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/golden"
	"github.com/connerohnesorge/goonix/parser"
)

func TestHelloDrvPathMatchesGolden(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		root = filepath.Dir(root)
	}

	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	if _, err := os.Stat(nixpkgs); err != nil {
		t.Skip("testdata/nixpkgs not present")
	}

	helloNix := filepath.Join(root, "testdata", "ladder", "hello.nix")
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	ev.SetCurFile(helloNix)

	expr, err := parser.ParseFile(helloNix)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}

	drvPath, err := extractDrvPath(ev, val)
	if err != nil {
		t.Fatal(err)
	}
	drvPath = strings.TrimSpace(drvPath) + "\n"
	golden.CompareFile(t, "hello.drvPath", []byte(drvPath))
}

var drvPathRe = regexp.MustCompile(`(/nix/store/[a-z0-9]+-hello-[^/\s]+\.drv)`)

func extractDrvPath(ev *eval.Evaluator, val *eval.Value) (string, error) {
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	sym := ev.Intern.Intern("drvPath")
	if b := fv.Attrs.Binds[sym]; b != nil {
		var raw *eval.Value
		if b.Thunk != nil {
			raw = eval.Deferred(b.Thunk)
		} else {
			raw = b.Value
		}
		dv, err := ev.ForceValue(raw)
		if err != nil {
			return "", err
		}
		if dv.Kind == eval.VKPath && dv.Path != "" {
			return dv.Path, nil
		}
	}
	text := fv.DisplayString(ev, ev.Intern)
	if m := drvPathRe.FindString(text); m != "" {
		return m, nil
	}
	return "", errMissing("drvPath on hello derivation")
}

type errMissing string

func (e errMissing) Error() string { return string(e) }
