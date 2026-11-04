package store_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/connerohnesorge/goonix/store"
)

func TestATermRoundTripMatchesNixStoreFile(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
	path := "/nix/store/jhnz6wx6p5h0pqykj1i7jdzj8ddqlgnf-bootstrap-stage0-stdenv-linux.drv"
	if _, err := os.Stat(path); err != nil {
		t.Skip("nix store drv not on disk")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	d, err := gonixdrv.ReadDerivation(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := d.WriteDerivation(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, buf.Bytes()) {
		t.Fatalf("go-nix WriteDerivation differs from Nix store file (%d vs %d bytes)", len(buf.Bytes()), len(raw))
	}
	gotPath, err := store.DrvPath(d)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := path
	if gotPath != wantPath {
		t.Fatalf("DrvPath from parsed ATerm %q want %q", gotPath, wantPath)
	}
}
