// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
)

func TestGetNameEmptySetErrors(t *testing.T) {
	lay := oracle.MustLayout(t)
	src := `with import <nixpkgs> { config = {}; overlays = []; }; lib.getName { }`
	ev := eval.NewEvaluator().WithEffects(eval.IOEffects{})
	ev.NixPath.PrependEntries("nixpkgs=" + lay.Nixpkgs)
	ev.NixPath.PrependEntries("nix=" + lay.Nix)
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ev.EvalWithBuiltins(expr)
	if err == nil {
		t.Fatal("expected error for getName {}")
	}
	if !strings.Contains(err.Error(), "name") {
		t.Fatalf("go error=%v", err)
	}
}
