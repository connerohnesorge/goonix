package eval_test

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

type bootStageRow struct {
	CcType         string  `json:"ccType"`
	StdenvName     *string `json:"stdenvName"`
	PrevStdenvName *string `json:"prevStdenvName"`
	StageTag       *string `json:"stageTag,omitempty"`
	PrevSeen       *string `json:"prevSeen,omitempty"`
}

func evalOracleNix(t *testing.T, path string) map[string]any {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not available")
	}
	out, err := exec.Command("nix-instantiate", "--eval", "--strict", "--json", "-E", "import "+path+" {}").Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.Fatalf("nix: %s\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return v
}

func evalGoonixFile(t *testing.T, importPath string) map[string]any {
	t.Helper()
	root := modRoot(t)
	abs, _ := filepath.Abs(importPath)
	src := `import "` + abs + `" {}`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	return valueToJSONMap(t, ev, val)
}

func evalGoonixFileAllowErr(t *testing.T, importPath string) (map[string]any, error) {
	t.Helper()
	root := modRoot(t)
	abs, _ := filepath.Abs(importPath)
	src := `import "` + abs + `" {}`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	nixpkgs := filepath.Join(root, "testdata", "nixpkgs")
	ev.NixPath.PrependEntries("nixpkgs=" + nixpkgs)
	ev.NixPath.PrependEntries("nix=" + filepath.Join(root, "testdata", "nix"))
	expr, err := parser.ParseString(src)
	if err != nil {
		return nil, err
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		return nil, err
	}
	return valueToJSONMapAllowErr(t, ev, val)
}

func valueToJSONMapAllowErr(t *testing.T, ev *eval.Evaluator, v *eval.Value) (map[string]any, error) {
	t.Helper()
	fv, err := ev.ForceValue(v)
	if err != nil {
		return nil, err
	}
	m, err := evalValueToJSONAllowErr(t, ev, fv)
	if err != nil {
		return nil, err
	}
	out, ok := m.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected attrset JSON")
	}
	return out, nil
}

func evalValueToJSONAllowErr(t *testing.T, ev *eval.Evaluator, v *eval.Value) (any, error) {
	t.Helper()
	if v == nil {
		return nil, nil
	}
	fv, err := ev.ForceValue(v)
	if err != nil {
		return nil, err
	}
	switch fv.Kind {
	case eval.VKNull:
		return nil, nil
	case eval.VKBool:
		return fv.Bool, nil
	case eval.VKInt:
		return fv.Int, nil
	case eval.VKString:
		return fv.String, nil
	case eval.VKList:
		out := make([]any, len(fv.List))
		for i, el := range fv.List {
			x, err := evalValueToJSONAllowErr(t, ev, el)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case eval.VKAttrs:
		m := map[string]any{}
		for sym, bind := range fv.Attrs.Binds {
			name := ev.Intern.Lookup(sym)
			var val *eval.Value
			if bind.Thunk != nil {
				val, err = ev.ForceValue(eval.Deferred(bind.Thunk))
			} else {
				val = bind.Value
			}
			if err != nil {
				return nil, err
			}
			x, err := evalValueToJSONAllowErr(t, ev, val)
			if err != nil {
				return nil, err
			}
			m[name] = x
		}
		return m, nil
	default:
		return fv.DisplayString(ev, ev.Intern), nil
	}
}

func valueToJSONMap(t *testing.T, ev *eval.Evaluator, v *eval.Value) map[string]any {
	t.Helper()
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := evalValueToJSON(t, ev, fv).(map[string]any)
	if !ok {
		t.Fatalf("expected attrset, got %T", evalValueToJSON(t, ev, fv))
	}
	return m
}

func evalValueToJSON(t *testing.T, ev *eval.Evaluator, v *eval.Value) any {
	t.Helper()
	if v == nil {
		return nil
	}
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	switch fv.Kind {
	case eval.VKNull:
		return nil
	case eval.VKBool:
		return fv.Bool
	case eval.VKInt:
		return fv.Int
	case eval.VKString:
		return fv.String
	case eval.VKList:
		out := make([]any, len(fv.List))
		for i, el := range fv.List {
			out[i] = evalValueToJSON(t, ev, el)
		}
		return out
	case eval.VKAttrs:
		m := map[string]any{}
		for sym, bind := range fv.Attrs.Binds {
			name := ev.Intern.Lookup(sym)
			var val *eval.Value
			if bind.Thunk != nil {
				val, err = ev.ForceValue(eval.Deferred(bind.Thunk))
			} else {
				val = bind.Value
			}
			if err != nil {
				t.Fatal(err)
			}
			m[name] = evalValueToJSON(t, ev, val)
		}
		return m
	default:
		return fv.DisplayString(ev, ev.Intern)
	}
}

func parseChain(t *testing.T, raw any) []bootStageRow {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var rows []bootStageRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func rowEqual(a, b bootStageRow) bool {
	return a.CcType == b.CcType &&
		ptrStr(a.StdenvName) == ptrStr(b.StdenvName) &&
		ptrStr(a.PrevStdenvName) == ptrStr(b.PrevStdenvName) &&
		ptrStr(a.StageTag) == ptrStr(b.StageTag) &&
		ptrStr(a.PrevSeen) == ptrStr(b.PrevSeen)
}

func TestDfoldNineStageOrderMatchesNixOracle(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "dfold_nine_stage_order.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)

	nixByIdx := parseChain(t, nixV["chainByDfoldIndex"])
	goByIdx := parseChain(t, goV["chainByDfoldIndex"])
	if len(nixByIdx) != len(goByIdx) {
		t.Fatalf("stage count nix=%d go=%d", len(nixByIdx), len(goByIdx))
	}
	for i := range nixByIdx {
		if !rowEqual(nixByIdx[i], goByIdx[i]) {
			t.Fatalf("first diff at dfold index %d:\nnix=%+v\ngo=%+v", i, nixByIdx[i], goByIdx[i])
		}
	}
}

func TestDfoldStageOrderMatchesNixOracle(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "dfold_stage_order.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)

	nixChain := parseChain(t, nixV["chain"])
	goChain := parseChain(t, goV["chain"])
	if len(nixChain) != len(goChain) {
		t.Fatalf("chain length nix=%d go=%d\nnix=%+v\ngo=%+v", len(nixChain), len(goChain), nixChain, goChain)
	}
	for i := range nixChain {
		if !rowEqual(nixChain[i], goChain[i]) {
			t.Fatalf("first diff at chain[%d]:\nnix=%+v\ngo=%+v", i, nixChain[i], goChain[i])
		}
	}
	if nixV["finalStdenvName"] != goV["finalStdenvName"] {
		t.Fatalf("finalStdenvName nix=%v go=%v", nixV["finalStdenvName"], goV["finalStdenvName"])
	}
	if nixV["finalCcType"] != goV["finalCcType"] {
		t.Fatalf("finalCcType nix=%v go=%v", nixV["finalCcType"], goV["finalCcType"])
	}
}

func TestLinuxBootStageDfoldStubMatchesNixOracle(t *testing.T) {
	t.Skip("linux boot chain walk forces stage asserts; use dfold_nine_stage_order + import tryEval probe until tryEval walk oracle lands")
}

func evalOracleNixOptional(path string) (map[string]any, error) {
	out, err := exec.Command("nix-instantiate", "--eval", "--strict", "--json", "-E", "import "+path+" {}").Output()
	if err != nil {
		return nil, err
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func TestLinuxBootStageChainMatchesNixOracle(t *testing.T) {
	t.Skip("oracle fixture linux_boot_stage_chain.nix fails on pinned nixpkgs (allowAliases / booter); use bootstrap ATerm ladder instead")
}

func ptrStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func TestLinuxBootStageChainImportPath(t *testing.T) {
	t.Skip("linux_boot_stage_chain.nix oracle fails on pinned nixpkgs; bootstrap ladder covers stage chain")
}
