package builder

import (
	"strings"
	"testing"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/store"
)

// The spawn of a native (Windows) build follows NovaNix: "/nix/store/" in
// the builder, arguments and environment becomes the physical store
// directory and '/', output variables are the physical output paths, and
// only a few host variables survive.
func TestPrepareNativeSpawnMatchesNovaNix(t *testing.T) {
	const storeDir = `C:\nix\store`
	out := "/nix/store/0123456789abcdfghijklmnpqrsvwxyz-pkg"
	d := &gonixdrv.Derivation{
		Builder:   "/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bash/usr/bin/bash.exe",
		Arguments: []string{"-e", "/nix/store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-setup.sh"},
		Env: map[string]string{
			"out":        out,
			"src":        "/nix/store/cccccccccccccccccccccccccccccccc-src.tar.gz",
			"self":       store.OutputPlaceholder("out"),
			"bare":       "/nix/store",
			"TMPDIR":     "override",
			"passAsFile": "big",
			"big":        "see /nix/store/dddddddddddddddddddddddddddddddd-x",
		},
		Outputs: map[string]*gonixdrv.Output{"out": {Path: out}},
	}
	physOut := storeDir + `\0123456789abcdfghijklmnpqrsvwxyz-pkg`
	b := &Builder{}
	sp, err := b.prepareNativeSpawn(d, map[string]string{"out": out}, map[string]string{"out": physOut}, storeDir, `C:\Temp`,
		[]string{"SystemRoot=C:\\Windows", "PATH=C:\\evil", "HOME=C:\\Users\\x", "windir=C:\\Windows", "MSYS=winsymlinks"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `C:\nix\store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bash/usr/bin/bash.exe`; sp.Builder != want {
		t.Errorf("builder %q, want %q", sp.Builder, want)
	}
	if want := []string{"-e", `C:\nix\store/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-setup.sh`}; strings.Join(sp.Args, "|") != strings.Join(want, "|") {
		t.Errorf("args %q, want %q", sp.Args, want)
	}
	if want := `C:\Temp\0123456789abcdfghijklmnpqrsvwxyz`; sp.BuildDir != want {
		t.Errorf("build dir %q, want %q", sp.BuildDir, want)
	}
	env := map[string]string{}
	for _, kv := range sp.Env {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	want := map[string]string{
		"out":               physOut,
		"src":               `C:\nix\store/cccccccccccccccccccccccccccccccc-src.tar.gz`,
		"self":              physOut,
		"bare":              "/nix/store",
		"TMPDIR":            "override",
		"TMP":               sp.BuildDir,
		"TEMP":              sp.BuildDir,
		"TEMPDIR":           sp.BuildDir,
		"NIX_BUILD_TOP":     sp.BuildDir,
		"USERPROFILE":       sp.BuildDir,
		"NIX_STORE":         storeDir,
		"SOURCE_DATE_EPOCH": "315532800",
		"PATH":              `C:\nix\store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bash/usr/bin;C:\nix\store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-bash/usr\usr\bin;C:\Windows\System32;C:\Windows`,
		"SystemRoot":        `C:\Windows`,
		"windir":            `C:\Windows`,
		"COMSPEC":           `C:\Windows\System32\cmd.exe`,
		"PATHEXT":           ".COM;.EXE;.BAT;.CMD",
		"bigPath":           sp.BuildDir + `\` + passAsFileName("big"),
		"passAsFile":        "big",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s=%q, want %q", k, env[k], v)
		}
	}
	for _, k := range []string{"HOME", "MSYS", "big"} {
		if _, ok := env[k]; ok {
			t.Errorf("%s leaked into the build environment", k)
		}
	}
	if len(env) != len(want) {
		t.Errorf("env has %d variables, want %d: %v", len(env), len(want), sp.Env)
	}
	if got := sp.Files[passAsFileName("big")]; got != `see C:\nix\store/dddddddddddddddddddddddddddddddd-x` {
		t.Errorf("passAsFile body %q", got)
	}
}
