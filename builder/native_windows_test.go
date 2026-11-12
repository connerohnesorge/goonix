//go:build windows

package builder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/store/local"
)

// A cmd.exe /c builder runs through the shell with its arguments joined, as
// NovaNix runs it, and writes the physical output path it is given.
func TestNativeCmdBuilderWritesOut(t *testing.T) {
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	err = realiseAttrs(t, st, `{
	  name = "cmd-out";
	  system = "x86_64-windows";
	  builder = "C:\\Windows\\System32\\cmd.exe";
	  args = [ "/c" "echo hi> %out%" ];
	}`, false)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(st.Root, "store", "*-cmd-out"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("outputs %v %v", matches, err)
	}
	body, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "hi\r\n" {
		t.Fatalf("out %q", body)
	}
}

// A builder that exits zero without writing $out fails the build.
func TestNativeBuilderMissingOutputFails(t *testing.T) {
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	err = realiseAttrs(t, st, `{
	  name = "no-out";
	  system = "x86_64-windows";
	  builder = "C:\\Windows\\System32\\cmd.exe";
	  args = [ "/c" "exit 0" ];
	}`, false)
	if err == nil || !strings.Contains(err.Error(), "failed to produce output path for output 'out'") {
		t.Fatalf("err %v", err)
	}
}

// A derivation for another system is refused before anything runs.
func TestNativeBuilderRefusesForeignSystem(t *testing.T) {
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	err = realiseAttrs(t, st, `{
	  name = "linux-only";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "echo hi > $out" ];
	}`, false)
	if err == nil || !strings.Contains(err.Error(), "a 'x86_64-linux' with features {} is required") {
		t.Fatalf("err %v", err)
	}
}
