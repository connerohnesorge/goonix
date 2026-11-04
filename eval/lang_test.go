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
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/parser"
)

// LangCase is one language conformance test (skeleton for future lang/ fixtures).
type LangCase struct {
	Name       string
	Source     string
	WantErr    bool
	WantSubstr string
}

// RunLangCases executes cases with builtins in scope.
func RunLangCases(t *testing.T, cases []LangCase) {
	t.Helper()
	ev := eval.NewEvaluator()
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			expr, err := parser.ParseString(c.Source)
			if err != nil {
				t.Fatal(err)
			}
			val, err := ev.EvalWithBuiltins(expr)
			if c.WantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.WantSubstr != "" {
				got := val.DisplayString(ev, ev.Intern)
				if !contains(got, c.WantSubstr) {
					t.Fatalf("got %q, want substring %q", got, c.WantSubstr)
				}
			}
		})
	}
}

func TestLangSkeleton(t *testing.T) {
	RunLangCases(t, []LangCase{
		{Name: "add", Source: `1 + 2`, WantSubstr: "3"},
		{Name: "builtins.toString", Source: `builtins.toString 42`, WantSubstr: "42"},
		{Name: "builtins.hasAttr", Source: `builtins.hasAttr "nixVersion" builtins`, WantSubstr: "true"},
		{Name: "builtins ? nixVersion", Source: `if builtins ? nixVersion then "yes" else "no"`, WantSubstr: "yes"},
		{Name: "compareVersions", Source: `builtins.toString (builtins.compareVersions "2.0" "1.9")`, WantSubstr: "1"},
	})
}

func TestNixPathFindFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.nix"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	np := eval.ParseNixPath("").WithSearchPath(dir)
	p, err := np.FindFile("hello")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(p, "hello.nix") {
		t.Fatalf("path = %q", p)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(sub) > 0 && searchSub(s, sub)))
}

func searchSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
