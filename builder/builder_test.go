// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/builder"
	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestTopoSortOrder(t *testing.T) {
	load := func(p string) (*gonixdrv.Derivation, error) {
		switch p {
		case "/nix/store/root.drv":
			return &gonixdrv.Derivation{InputDerivations: map[string][]string{"/nix/store/aaa.drv": {"out"}}}, nil
		case "/nix/store/aaa.drv":
			return &gonixdrv.Derivation{}, nil
		default:
			return nil, os.ErrNotExist
		}
	}
	order, err := builder.TopoSort("/nix/store/root.drv", load)
	if err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "/nix/store/aaa.drv" {
		t.Fatalf("order=%v", order)
	}
	if err := builder.ValidateTopo(order, load); err != nil {
		t.Fatal(err)
	}
}

func TestBuildTrivialShAndToFileDep(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	root := t.TempDir()
	st, err := local.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	docPath, err := st.AddToStoreText("builder-doc", "payload\n")
	if err != nil {
		t.Fatal(err)
	}

	textDrv := &gonixdrv.Derivation{
		Outputs: map[string]*gonixdrv.Output{
			"out": {Path: store.OutputPlaceholder("out")},
		},
		InputSources: []string{docPath},
		Platform:     "x86_64-linux",
		Builder:      "/bin/sh",
		Arguments:    []string{"-c", "/bin/cat " + docPath + " > $out/doc"},
		Env: map[string]string{
			"name":    "with-doc",
			"out":     store.OutputPlaceholder("out"),
			"builder": "/bin/sh",
			"system":  "x86_64-linux",
		},
	}
	textDrvPath, err := writeDrv(st, textDrv, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}

	trivial := &gonixdrv.Derivation{
		Outputs: map[string]*gonixdrv.Output{
			"out": {Path: store.OutputPlaceholder("out")},
		},
		InputDerivations: map[string][]string{textDrvPath: {"out"}},
		Platform:         "x86_64-linux",
		Builder:          "/bin/sh",
		Arguments:        []string{"-c", "echo ok > $out/status"},
		Env: map[string]string{
			"name":    "trivial",
			"out":     store.OutputPlaceholder("out"),
			"builder": "/bin/sh",
			"system":  "x86_64-linux",
		},
	}
	repl := map[string]string{}
	textRepl, err := gonixdrv.ReadDerivation(bytes.NewReader(mustReadDrv(st, textDrvPath)))
	if err != nil {
		t.Fatal(err)
	}
	tr, err := textRepl.CalculateDrvReplacement(repl)
	if err != nil {
		t.Fatal(err)
	}
	repl[textDrvPath] = tr

	rootDrv, err := writeDrv(st, trivial, repl)
	if err != nil {
		t.Fatal(err)
	}

	b := &builder.Builder{Store: st}
	if err := b.BuildRealisation(rootDrv); err != nil {
		t.Fatal(err)
	}
	d, err := gonixdrv.ReadDerivation(bytes.NewReader(mustReadDrv(st, rootDrv)))
	if err != nil {
		t.Fatal(err)
	}
	outs, err := d.CalculateOutputPaths(repl)
	if err != nil {
		t.Fatal(err)
	}
	outPath := outs["out"]
	ok, err := st.IsValidPath(outPath)
	if err != nil || !ok {
		t.Fatalf("out valid=%v err=%v path=%s", ok, err, outPath)
	}
	phys := local.PhysicalPath(root, outPath)
	gotHash, _, err := local.NarHashAndSize(phys)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("nix-store"); err == nil {
		out, err := exec.Command("nix-store", "--dump", phys).Output()
		if err != nil {
			t.Fatalf("nix-store --dump: %v", err)
		}
		want, _, err := local.HashNARStream(bytes.NewReader(out))
		if err != nil {
			t.Fatal(err)
		}
		if gotHash != want {
			t.Fatalf("nar hash go=%s nix=%s", gotHash, want)
		}
	}
}

func TestBuildEchoHiMatchesNixBuild(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	root := t.TempDir()
	st, err := local.OpenStore(root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	attrs := `{
	  name = "t";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "echo hi > $out" ];
	}`
	if _, err := exec.LookPath("nix-build"); err != nil {
		t.Skip("no nix-build")
	}
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	expr, err := parser.ParseString(`builtins.derivationStrict ` + attrs)
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
	drvPath, err := eval.ForceStrictDrvPath(ev, fv)
	if err != nil {
		t.Fatal(err)
	}
	drv, err := ev.AttrsToDerivation(mustParseAttrs(t, ev, attrs))
	if err != nil {
		t.Fatal(err)
	}
	drv.DrvPath = drvPath
	var buf bytes.Buffer
	gd, err := writeGoNixFromEval(t, ev, drv, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.WriteDerivation(buf.Bytes(), drvPath); err != nil {
		t.Fatal(err)
	}
	b := &builder.Builder{Store: st}
	if err := b.BuildRealisation(drvPath); err != nil {
		t.Fatal(err)
	}
	repl := map[string]string{}
	outs, err := gd.CalculateOutputPaths(repl)
	if err != nil {
		t.Fatal(err)
	}
	outPath := outs["out"]
	phys := local.PhysicalPath(st.Root, outPath)
	gotHash, _, err := local.NarHashAndSize(phys)
	if err != nil {
		t.Fatal(err)
	}
	nixStore := filepath.Join(root, "store")
	nixOut, err := exec.Command("nix-build", "--store", nixStore, "-E", "derivation "+attrs).Output()
	if err != nil {
		t.Skipf("nix-build in temp store: %v", err)
	}
	nixOutPath := strings.TrimSpace(string(nixOut))
	nixPhys := filepath.Join(nixStore, filepath.Base(nixOutPath))
	wantHash, _, err := local.NarHashAndSize(nixPhys)
	if err != nil {
		t.Fatal(err)
	}
	if gotHash != wantHash {
		t.Fatalf("nar hash go=%s nix=%s", gotHash, wantHash)
	}
}

func mustParseAttrs(t *testing.T, ev *eval.Evaluator, src string) *eval.Value {
	t.Helper()
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func writeGoNixFromEval(t *testing.T, ev *eval.Evaluator, drv *eval.DerivationValue, buf *bytes.Buffer) (*gonixdrv.Derivation, error) {
	t.Helper()
	aterm, err := eval.SerializeDerivation(store.Config{StoreDir: ev.Effects.StoreDir()}, drv)
	if err != nil {
		return nil, err
	}
	buf.Write(aterm)
	return gonixdrv.ReadDerivation(bytes.NewReader(aterm))
}

func writeDrv(st *local.Store, d *gonixdrv.Derivation, repl map[string]string) (string, error) {
	outPaths, err := d.CalculateOutputPaths(repl)
	if err != nil {
		return "", err
	}
	for name, p := range outPaths {
		if o, ok := d.Outputs[name]; ok {
			o.Path = p
		}
	}
	if o, ok := d.Env["out"]; ok && o == store.OutputPlaceholder("out") {
		d.Env["out"] = outPaths["out"]
	}
	var buf bytes.Buffer
	if err := d.WriteDerivation(&buf); err != nil {
		return "", err
	}
	p, err := d.DrvPath()
	if err != nil {
		return "", err
	}
	if err := st.WriteDerivation(buf.Bytes(), p); err != nil {
		return "", err
	}
	return p, nil
}

func mustReadDrv(st *local.Store, p string) []byte {
	b, err := os.ReadFile(local.PhysicalPath(st.Root, p))
	if err != nil {
		panic(err)
	}
	return b
}
