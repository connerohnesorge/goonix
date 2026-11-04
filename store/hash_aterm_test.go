package store_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/store"
)

func TestWriteDerivationForHashMatchesGoNix(t *testing.T) {
	d := &gonixdrv.Derivation{
		Outputs: map[string]*gonixdrv.Output{
			"out": {Path: "/nix/store/abc-out", HashAlgorithm: "", Hash: ""},
		},
		InputDerivations: map[string][]string{},
		InputSources:     []string{},
		Platform:         "x86_64-linux",
		Builder:          "/bin/sh",
		Arguments:        []string{},
		Env: map[string]string{
			"builder": "/bin/sh",
			"name":    "t",
			"out":     "/nix/store/abc-out",
			"system":  "x86_64-linux",
		},
	}
	var ours bytes.Buffer
	if err := store.WriteDerivationForHash(&ours, d, false, nil); err != nil {
		t.Fatal(err)
	}
	var theirs bytes.Buffer
	if err := d.WriteDerivation(&theirs); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ours.Bytes(), theirs.Bytes()) {
		t.Fatalf("ATerm mismatch\nours=%q\ntheirs=%q", ours.Bytes(), theirs.Bytes())
	}
	sum := sha256.Sum256(ours.Bytes())
	t.Log("hash", hex.EncodeToString(sum[:]))
}
