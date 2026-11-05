// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

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
	body := readOnlyOutput(t, st)
	if body != "1000\n" {
		t.Fatalf("builder uid file %q, want 1000", body)
	}
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
