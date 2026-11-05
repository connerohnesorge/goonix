// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nix-community/go-nix/pkg/nixhash"

	"github.com/connerohnesorge/goonix/store/local"
)

func TestSandboxHidesHostAndRunsAsNixbld(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if !userMountNSAvailable() {
		t.Skip("user namespace unavailable")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	before := mountMarkers()
	extra, err := st.AddToStoreText("not-an-input", "secret\n")
	if err != nil {
		t.Fatal(err)
	}
	attrs := `{
	  name = "sandboxed";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" ''
	    if [ -e /usr/bin/id ]; then echo id-visible; exit 1; fi
	    if [ -e /homeless-shelter ]; then echo shelter; exit 1; fi
	    if [ "$PWD" != /build ]; then printf 'pwd=%s\n' "$PWD"; exit 1; fi
	    if [ "$NIX_BUILD_TOP" != /build ]; then printf 'top=%s\n' "$NIX_BUILD_TOP"; exit 1; fi
	    found=
	    while IFS= read -r line; do
	      case $line in
	        *enp*|*eth*|*ens*|*docker*|*wl*) found=1 ;;
	      esac
	    done < /proc/net/dev
	    if [ -n "$found" ]; then echo net; exit 1; fi
	    if [ -e .goonix-mount-ok ]; then echo marker-visible; exit 1; fi
	    if [ -e ` + extra + ` ]; then echo closure-leak; exit 1; fi
	    uid=
	    while IFS= read -r line; do
	      case $line in
	        Uid:*) set -- $line; uid=$2 ;;
	      esac
	    done < /proc/self/status
	    printf '%s\n' "$uid" > "$out"
	  '' ];
	}`
	if err := realiseAttrs(t, st, attrs, false); err != nil {
		t.Fatal(err)
	}
	body := readStoreFileSkipping(t, st, extra)
	if body != "1000\n" {
		t.Fatalf("builder uid file %q, want 1000", body)
	}
	for m := range mountMarkers() {
		if !before[m] {
			t.Fatalf("mount marker left behind: %s", m)
		}
	}
}

func TestFixedOutputBuildKeepsNetwork(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if !userMountNSAvailable() {
		t.Skip("user namespace unavailable")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	body := []byte("net\n")
	sum := sha256.Sum256(body)
	sri := nixhash.MustNewHash(nixhash.SHA256, sum[:]).Format(nixhash.SRI, true)
	attrs := fmt.Sprintf(`{
	  name = "fodnet";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" ''
	    found=
	    while IFS= read -r line; do
	      case $line in
	        *enp*|*eth*|*ens*|*docker*|*wl*) found=1 ;;
	      esac
	    done < /proc/net/dev
	    if [ -z "$found" ]; then
	      printf isolated > "$out"
	      exit 1
	    fi
	    printf 'net\n' > "$out"
	  '' ];
	  outputHashMode = "flat";
	  outputHashAlgo = "sha256";
	  outputHash = "%s";
	}`, sri)
	if err := realiseAttrs(t, st, attrs, false); err != nil {
		t.Fatal(err)
	}
	if got := readOnlyOutput(t, st); got != "net\n" {
		t.Fatalf("fixed-output network probe %q", got)
	}
}

func mountMarkers() map[string]bool {
	out := map[string]bool{}
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "*.goonix-mount-ok"))
	for _, m := range matches {
		out[m] = true
	}
	return out
}

func TestFailedBuildDeletesPartialOutput(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if !userMountNSAvailable() {
		t.Skip("user namespace unavailable")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	attrs := `{
	  name = "partial";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "printf partial > \"$out\"; exit 1" ];
	}`
	if err := realiseAttrs(t, st, attrs, false); err == nil {
		t.Fatal("failing builder succeeded")
	}
	entries, err := os.ReadDir(st.Root + "/store")
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".drv") {
			continue
		}
		t.Fatalf("partial output left behind: %s", ent.Name())
	}
}

func readStoreFileSkipping(t *testing.T, st *local.Store, skip string) string {
	t.Helper()
	entries, err := os.ReadDir(st.Root + "/store")
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".drv") || ent.Name() == filepath.Base(skip) {
			continue
		}
		body, err = os.ReadFile(st.Root + "/store/" + ent.Name())
		if err != nil {
			t.Fatal(err)
		}
	}
	if body == nil {
		t.Fatal("no build output")
	}
	return string(body)
}

func readOnlyOutput(t *testing.T, st *local.Store) string {
	t.Helper()
	entries, err := os.ReadDir(st.Root + "/store")
	if err != nil {
		t.Fatal(err)
	}
	var body []byte
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".drv") {
			continue
		}
		body, err = os.ReadFile(st.Root + "/store/" + ent.Name())
		if err != nil {
			t.Fatal(err)
		}
	}
	if body == nil {
		t.Fatal("no build output")
	}
	return string(body)
}
