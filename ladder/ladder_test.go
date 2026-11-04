package ladder_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/golden"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func ladderRoot(t *testing.T) string {
	t.Helper()
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
	return root
}

func evalLadderDrvPath(t *testing.T, nixFile string) string {
	t.Helper()
	lay := oracle.MustLayout(t)
	want := lay.EvalFileRaw(t, nixFile)

	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	ev.SetCurFile(nixFile)
	expr, err := parser.ParseFile(nixFile)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	got, err := forceDrvPathString(t, ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", want, got)
	}
	return got
}

func TestHelloDrvPathMatchesGolden(t *testing.T) {
	got := evalLadderDrvPath(t, filepath.Join(ladderRoot(t), "testdata", "ladder", "hello.nix"))
	got = strings.TrimSpace(got) + "\n"
	golden.CompareFile(t, "hello.drvPath", []byte(got))
}

func TestInputDrvLadderMatchesGolden(t *testing.T) {
	got := evalLadderDrvPath(t, filepath.Join(ladderRoot(t), "testdata", "ladder", "input_drv.nix"))
	got = strings.TrimSpace(got) + "\n"
	golden.CompareFile(t, "input_drv.drvPath", []byte(got))
}

func TestToFileLadderMatchesGolden(t *testing.T) {
	got := evalLadderDrvPath(t, filepath.Join(ladderRoot(t), "testdata", "ladder", "tofile.nix"))
	got = strings.TrimSpace(got) + "\n"
	golden.CompareFile(t, "tofile.drvPath", []byte(got))
}

func TestStdenvNoCCLadderMatchesGolden(t *testing.T) {
	got := evalLadderDrvPath(t, filepath.Join(ladderRoot(t), "testdata", "ladder", "stdenv_nocc.nix"))
	got = strings.TrimSpace(got) + "\n"
	golden.CompareFile(t, "stdenv_nocc.drvPath", []byte(got))
}

func TestCurlLadderMatchesGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("curl instantiates a large closure")
	}
	got := evalLadderDrvPath(t, filepath.Join(ladderRoot(t), "testdata", "ladder", "curl.nix"))
	got = strings.TrimSpace(got) + "\n"
	golden.CompareFile(t, "curl.drvPath", []byte(got))
}

var drvPathRe = regexp.MustCompile(`(/nix/store/[a-z0-9]+-[^/\s]+\.drv)`)

func forceDrvPathString(t *testing.T, ev *eval.Evaluator, val *eval.Value) (string, error) {
	t.Helper()
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	switch fv.Kind {
	case eval.VKPath:
		return fv.Path, nil
	case eval.VKString:
		return strings.TrimSpace(fv.String), nil
	case eval.VKAttrs:
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
			if dv.Kind == eval.VKString {
				return strings.TrimSpace(dv.String), nil
			}
		}
		text := fv.DisplayString(ev, ev.Intern)
		if m := drvPathRe.FindString(text); m != "" {
			return m, nil
		}
	}
	return "", errMissing("drvPath")
}

type errMissing string

func (e errMissing) Error() string { return string(e) }
