// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/builder"
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
