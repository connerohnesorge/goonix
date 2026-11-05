package builder_test

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nix-community/go-nix/pkg/nixhash"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestFixedOutputFetchurlMatchesNixOutPath(t *testing.T) {
	cases := []struct {
		name string
		algo string
		mode string
		body []byte
	}{
		{name: "sha256-flat", algo: "sha256", mode: "flat", body: []byte("hello fod flat\n")},
		{name: "sha1-flat", algo: "sha1", mode: "flat", body: []byte("sha1 flat payload")},
		{name: "sha512-flat", algo: "sha512", mode: "flat", body: []byte("sha512 flat")},
		{name: "sha256-recursive", algo: "sha256", mode: "recursive", body: []byte("recursive tree root\n")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(tc.body)
			}))
			defer srv.Close()
			sri, err := fodOutputHashSRI(tc.algo, tc.mode, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			attrs := fmt.Sprintf(`{
			  name = "fod-%s";
			  system = "x86_64-linux";
			  builder = "builtin:fetchurl";
			  url = %q;
			  outputHashAlgo = %q;
			  outputHash = %q;
			  outputHashMode = %q;
			}`, tc.name, srv.URL, tc.algo, sri, tc.mode)
			lay := oracle.MustLayout(t)
			wantOut := lay.EvalStrictOutPath(t, attrs)
			st, err := local.OpenStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			goOut, err := buildStrictFOD(t, st, attrs)
			if err != nil {
				t.Fatal(err)
			}
			if goOut != wantOut {
				t.Fatalf("out path go=%q nix=%q", goOut, wantOut)
			}
		})
	}
}

func TestFixedOutputFileURLMatchesNixOutPath(t *testing.T) {
	body := []byte("file url fod payload")
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	sri, err := fodOutputHashSRI("sha256", "flat", body)
	if err != nil {
		t.Fatal(err)
	}
	attrs := fmt.Sprintf(`{
	  name = "fod-file";
	  system = "x86_64-linux";
	  builder = "builtin:fetchurl";
	  url = "file://%s";
	  outputHashAlgo = "sha256";
	  outputHash = %q;
	  outputHashMode = "flat";
	}`, path, sri)
	lay := oracle.MustLayout(t)
	wantOut := lay.EvalStrictOutPath(t, attrs)
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	goOut, err := buildStrictFOD(t, st, attrs)
	if err != nil {
		t.Fatal(err)
	}
	if goOut != wantOut {
		t.Fatalf("out path go=%q nix=%q", goOut, wantOut)
	}
}

func buildStrictFOD(t *testing.T, st *local.Store, attrs string) (string, error) {
	t.Helper()
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(`builtins.derivationStrict ` + attrs)
	if err != nil {
		return "", err
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		return "", err
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		return "", err
	}
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		return "", err
	}
	attrsV, err := parser.ParseString(attrs)
	if err != nil {
		return "", err
	}
	av, err := ev.EvalWithBuiltins(attrsV)
	if err != nil {
		return "", err
	}
	av, err = ev.ForceValue(av)
	if err != nil {
		return "", err
	}
	drv, err := ev.AttrsToDerivation(av)
	if err != nil {
		return "", err
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	if err := eval.PrepareDerivationPaths(ev, cfg, drv); err != nil {
		return "", err
	}
	aterm, err := eval.SerializeDerivation(cfg, drv)
	if err != nil {
		return "", err
	}
	if err := st.WriteDerivation(aterm, drvPath); err != nil {
		return "", err
	}
	b := &builder.Builder{Store: st, Config: builder.Config{AllowRelocated: true}}
	if err := b.BuildRealisation(drvPath); err != nil {
		return "", err
	}
	return drv.OutPaths["out"], nil
}

func fodOutputHashSRI(algo, mode string, body []byte) (string, error) {
	if mode == "recursive" || mode == "nar" {
		dir, err := os.MkdirTemp("", "fod-nar-")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(dir)
		if err := os.WriteFile(filepath.Join(dir, "downloaded"), body, 0o644); err != nil {
			return "", err
		}
		narB32, _, err := local.NarHashAndSize(dir)
		if err != nil {
			return "", err
		}
		raw, err := nixhash.ParseAny("sha256:"+narB32, nil)
		if err != nil {
			return "", err
		}
		return raw.Hash.Format(nixhash.SRI, false), nil
	}
	var h *nixhash.Hash
	switch algo {
	case "sha256":
		sum := sha256.Sum256(body)
		h = nixhash.MustNewHash(nixhash.SHA256, sum[:])
	case "sha1":
		sum := sha1.Sum(body)
		h = nixhash.MustNewHash(nixhash.SHA1, sum[:])
	case "sha512":
		sum := sha512.Sum512(body)
		h = nixhash.MustNewHash(nixhash.SHA512, sum[:])
	default:
		return "", fmt.Errorf("unsupported algo %q", algo)
	}
	return h.Format(nixhash.SRI, false), nil
}
