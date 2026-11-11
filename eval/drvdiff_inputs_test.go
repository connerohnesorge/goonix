package eval_test

import (
	"path/filepath"
	"testing"
)

func TestStdenvProbeRegisteredTopInputDrvs(t *testing.T) {
	root := modRoot(t)
	goPath, ev := evalProbeDrvPath(t, filepath.Join(root, "testdata", "oracle", "stdenv_nocc_mkderivation_probe.nix"))
	d, ok := ev.RegisteredDerivation(goPath)
	if !ok {
		t.Fatal("top not registered")
	}
	t.Logf("inputDrvs=%d inputSrcs=%d", len(d.InputDerivations), len(d.InputSources))
	t.Logf("env stdenv=%q builder=%q", d.Env["stdenv"], d.Env["builder"])
	for p, o := range d.InputDerivations {
		t.Logf("  drv %s %v", p, o)
	}
}
