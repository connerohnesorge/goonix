// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestSubstringNegativeLengthToEnd(t *testing.T) {
	ev := eval.NewEvaluator()
	expr, err := parser.ParseString(`builtins.substring 0 (-1) "gcc"`)
	if err != nil {
		t.Fatal(err)
	}
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	fv, err := ev.ForceValue(val)
	if err != nil {
		t.Fatal(err)
	}
	if fv.Kind != eval.VKString || fv.String != "gcc" {
		t.Fatalf("got %q", fv.String)
	}
}
