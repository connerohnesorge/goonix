// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval_test

import (
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

func TestDerivationStrictTrivial(t *testing.T) {
	src := `
builtins.derivationStrict {
  name = "goonix-test";
  system = "x86_64-linux";
  builder = "bash";
  args = [ "-c" "echo ok" ];
}
`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	val, err := ev.EvalWithBuiltins(expr)
	if err != nil {
		t.Fatal(err)
	}
	s := val.DisplayString(ev, ev.Intern)
	if !strings.Contains(s, "{") {
		t.Fatalf("expected attrset, got %q", s)
	}
	typeSym := ev.Intern.Intern("type")
	bind := val.Attrs.Binds[typeSym]
	if bind == nil || bind.Value == nil {
		t.Fatal("missing type")
	}
	if bind.Value.Kind != eval.VKString || bind.Value.String != "derivation" {
		t.Fatalf("type = %v", bind.Value)
	}
	drvSym := ev.Intern.Intern("drvPath")
	db := val.Attrs.Binds[drvSym]
	if db == nil || db.Value == nil {
		t.Fatal("missing drvPath")
	}
	if db.Value.Kind != eval.VKPath || !strings.HasPrefix(db.Value.Path, "/nix/store/") {
		t.Fatalf("drvPath = %v", db.Value)
	}
}

func TestInfiniteRecursionBlackhole(t *testing.T) {
	src := `let x = x; in x`
	expr, err := parser.ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	ev := eval.NewEvaluator()
	_, err = ev.Eval(expr)
	if err == nil {
		t.Fatal("expected infinite recursion error")
	}
	if !strings.Contains(err.Error(), "infinite recursion") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveLet(t *testing.T) {
	expr, err := parser.ParseString(`let a = 1; in a`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = eval.Resolve(expr, nil)
	if err != nil {
		t.Fatal(err)
	}
}
