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
	"runtime"
	"strconv"
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

func TestSandboxLoopbackCoresAndTerm(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if !userMountNSAvailable() {
		t.Skip("user namespace unavailable")
	}
	conf := t.TempDir()
	t.Setenv("NIX_CONF_DIR", conf)
	t.Setenv("NIX_CONFIG", "cores = 0\n")
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	script := `
	    found=
	    while IFS= read -r line; do
	      case $line in
	        *127.0.0.1*) found=1 ;;
	      esac
	    done < /proc/net/fib_trie
	    if [ -z "$found" ]; then
	      printf 'no-lo\n' > "$out"
	      exit 1
	    fi
	    printf '%s %s\n' "$TERM" "$NIX_BUILD_CORES" > "$out"
	`
	attrs := `{
	  name = "envprobe";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  TERM = "vt100";
	  args = [ "-c" ''` + script + `'' ];
	}`
	if err := realiseAttrs(t, st, attrs, false); err != nil {
		t.Fatal(err)
	}
	if body := readStoreFileSkipping(t, st, ""); body != "xterm-256color 0\n" {
		t.Fatalf("term/cores %q", body)
	}

	t.Setenv("NIX_CONFIG", "cores = 2\n")
	st2, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	drvCores := `{
	  name = "envprobe-drv";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  NIX_BUILD_CORES = "9";
	  args = [ "-c" "printf '%s\n' \"$NIX_BUILD_CORES\" > \"$out\"" ];
	}`
	if err := realiseAttrs(t, st2, drvCores, false); err != nil {
		t.Fatal(err)
	}
	if body := readStoreFileSkipping(t, st2, ""); body != "9\n" {
		t.Fatalf("drv cores %q", body)
	}

	t.Setenv("NIX_CONFIG", "sandbox = true\n")
	st3, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st3.Close()
	plain := `{
	  name = "envprobe-default";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "printf '%s\n' \"$NIX_BUILD_CORES\" > \"$out\"" ];
	}`
	if err := realiseAttrs(t, st3, plain, false); err != nil {
		t.Fatal(err)
	}
	want := strconv.Itoa(runtime.NumCPU()) + "\n"
	if body := readStoreFileSkipping(t, st3, ""); body != want {
		t.Fatalf("default cores %q want %q", body, want)
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
