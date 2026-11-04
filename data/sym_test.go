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

package data_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/data"
)

func TestInterner(t *testing.T) {
	in := data.NewInterner()
	a := in.Intern("foo")
	b := in.Intern("foo")
	c := in.Intern("bar")
	if a != b {
		t.Fatal("expected same symbol for identical strings")
	}
	if a == c {
		t.Fatal("expected different symbols")
	}
	if got := in.Lookup(a); got != "foo" {
		t.Fatalf("Lookup = %q, want foo", got)
	}
}

func TestValueConstructors(t *testing.T) {
	in := data.NewInterner()
	key := in.Intern("x")
	v := data.AttrSetV(map[data.Symbol]*data.Value{
		key: data.IntV(42),
	})
	if v.Kind != data.KindAttrSet {
		t.Fatalf("kind = %v, want AttrSet", v.Kind)
	}
	if v.Attrs[key].Int != 42 {
		t.Fatalf("attr value = %d, want 42", v.Attrs[key].Int)
	}
}
