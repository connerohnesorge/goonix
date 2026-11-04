package eval_test

import (
	"path/filepath"
	"reflect"
	"testing"
)

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	var norm func(v any) any
	norm = func(v any) any {
		switch x := v.(type) {
		case float64:
			if x == float64(int64(x)) {
				return int64(x)
			}
			return x
		case []any:
			out := make([]any, len(x))
			for i, el := range x {
				out[i] = norm(el)
			}
			return out
		case map[string]any:
			out := make(map[string]any, len(x))
			for k, el := range x {
				out[k] = norm(el)
			}
			return out
		default:
			return v
		}
	}
	return reflect.DeepEqual(norm(a), norm(b))
}

func TestListBootPrimitivesMatchNixOracle(t *testing.T) {
	root := modRoot(t)
	path := filepath.Join(root, "testdata", "oracle", "list_boot_primitives.nix")
	nixV := evalOracleNix(t, path)
	goV := evalGoonixFile(t, path)

	keys := []string{
		"lengthStages", "reverseListStages", "imap1Indexes", "imap1OnReversed",
		"genListIds", "initLen", "lastVal", "foldlPrime", "append", "dfoldLen",
		"orFalse", "orTrue", "elemAt0Type", "elemAtLastN",
	}
	for _, k := range keys {
		if !jsonEqual(t, nixV[k], goV[k]) {
			t.Fatalf("primitive %q differs:\nnix=%#v\ngo=%#v", k, nixV[k], goV[k])
		}
	}
}
