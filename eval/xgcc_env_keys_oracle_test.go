package eval_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXgccNoEmptyEnvKeyWithValue(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	goGW, _, ev := bootstrapStage2GCCWrapperPair(t)
	gw, ok := ev.RegisteredDerivation("/nix/store/" + goGW)
	if !ok {
		t.Fatal("gcc-wrapper not registered")
	}
	var goXgccKey string
	for p := range gw.InputDerivations {
		if strings.Contains(p, "xgcc-13.3.0") {
			goXgccKey = strings.TrimPrefix(p, "/nix/store/")
			break
		}
	}
	if goXgccKey == "" {
		t.Fatal("no xgcc input on gcc-wrapper")
	}
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goXgccKey)
	if !ok {
		t.Fatalf("xgcc %s not registered", goXgccKey)
	}
	if v, ok := gd.Env[""]; ok && v != "" {
		t.Fatalf("xgcc env has empty key with value %q", v)
	}
	for k, v := range gd.Env {
		if k == "" && v != "" {
			t.Fatalf("empty env key with value %q", v)
		}
	}
}

func TestBootstrapStage2GCCWrapperEnvContextInvariant(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	goGW, _, ev := bootstrapStage2GCCWrapperPair(t)
	gd, ok := ev.RegisteredDerivation("/nix/store/" + goGW)
	if !ok {
		t.Fatal("gcc-wrapper not registered")
	}
	for _, key := range []string{"cc", "bintools"} {
		s := gd.Env[key]
		if !strings.HasPrefix(s, "/nix/store/") {
			t.Fatalf("env[%s]=%q", key, s)
		}
		found := false
		for p := range gd.InputDerivations {
			if key == "cc" && !strings.Contains(p, "xgcc") {
				continue
			}
			if key == "bintools" && !strings.Contains(p, "binutils-wrapper") {
				continue
			}
			reg, ok := ev.RegisteredDerivation(p)
			if !ok {
				continue
			}
			if reg.Env["out"] == s {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("env[%s]=%q is not the out path of any matching inputDrv", key, s)
		}
	}
}

func TestStdenvDrvPathDeterminismMatchesNix(t *testing.T) {
	root := modRoot(t)
	probe := filepath.Join(root, "eval", "testdata", "stdenv_drvpath_determinism_oracle.nix")
	nixV := evalOracleNix(t, probe)
	goV := evalGoonixFile(t, probe)
	if !jsonEqual(t, nixV, goV) {
		t.Fatalf("stdenv drvPath determinism oracle\nnix=%#v\ngo=%#v", nixV, goV)
	}
}
