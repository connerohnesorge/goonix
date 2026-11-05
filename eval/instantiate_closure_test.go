// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"os"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

func TestInstantiateWritesDependencyDrvs(t *testing.T) {
	dir := t.TempDir()
	ev := NewEvaluator().WithEffects(IOEffects{Root: dir})
	expr := `let
  dep = builtins.derivationStrict { name = "dep"; builder = "/bin/sh"; system = "x86_64-linux"; };
in builtins.derivationStrict { name = "top"; builder = dep.out; system = "x86_64-linux"; }
`
	parsed, err := parser.ParseString(expr)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ev.ForceValue(val); err != nil {
		t.Fatal(err)
	}
	var top *DerivationValue
	for _, drv := range ev.drvStrictByPath {
		if drv.Name == "top" {
			top = drv
		}
	}
	if top == nil {
		t.Fatal("top derivation was not registered")
	}
	if len(top.InputDerivations) == 0 {
		t.Fatal("top derivation does not reference its dependency")
	}
	if err := ev.WriteDerivationClosure(store.Config{StoreDir: dir}, top); err != nil {
		t.Fatal(err)
	}
	sawDep := false
	for path := range top.InputDerivations {
		sawDep = true
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("dependency %s was not written: %v", path, err)
		}
		if !strings.HasSuffix(path, "-dep.drv") {
			t.Fatalf("dependency path %s", path)
		}
	}
	if !sawDep {
		t.Fatal("no dependency drv")
	}
	if _, err := os.Stat(top.DrvPath); err != nil {
		t.Fatalf("top drv: %v", err)
	}
}
