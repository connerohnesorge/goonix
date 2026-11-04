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

package data

import "testing"

// arenaHandle is a minimal bump-allocated value table used only for benchmarking.
type arenaHandle uint32

type arenaValue struct {
	kind  Kind
	intV  int64
	attrs map[Symbol]arenaHandle
}

type arena struct {
	slots []arenaValue
}

func (a *arena) alloc(v arenaValue) arenaHandle {
	a.slots = append(a.slots, v)
	return arenaHandle(len(a.slots) - 1)
}

func (a *arena) get(h arenaHandle) *arenaValue {
	return &a.slots[h]
}

func buildPointerAttrSet(in *Interner, n int) *Value {
	attrs := make(map[Symbol]*Value, n)
	for i := 0; i < n; i++ {
		key := in.Intern("attr")
		attrs[key] = IntV(int64(i))
	}
	return AttrSetV(attrs)
}

func buildArenaAttrSet(in *Interner, a *arena, n int) arenaHandle {
	attrs := make(map[Symbol]arenaHandle, n)
	for i := 0; i < n; i++ {
		key := in.Intern("attr")
		iv := a.alloc(arenaValue{kind: KindInt, intV: int64(i)})
		attrs[key] = iv
	}
	return a.alloc(arenaValue{kind: KindAttrSet, attrs: attrs})
}

func walkPointer(v *Value, depth int) int64 {
	if depth == 0 || v == nil {
		return 0
	}
	var sum int64
	if v.Kind == KindAttrSet {
		for _, child := range v.Attrs {
			sum += walkPointer(child, depth-1)
		}
	}
	if v.Kind == KindInt {
		sum += v.Int
	}
	return sum
}

func walkArena(a *arena, h arenaHandle, depth int) int64 {
	if depth == 0 {
		return 0
	}
	v := a.get(h)
	var sum int64
	if v.kind == KindAttrSet {
		for _, child := range v.attrs {
			sum += walkArena(a, child, depth-1)
		}
	}
	if v.kind == KindInt {
		sum += v.intV
	}
	return sum
}

func BenchmarkValuePointerBuild(b *testing.B) {
	in := NewInterner()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buildPointerAttrSet(in, 32)
	}
}

func BenchmarkValueArenaBuild(b *testing.B) {
	in := NewInterner()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var a arena
		buildArenaAttrSet(in, &a, 32)
	}
}

func BenchmarkValuePointerWalk(b *testing.B) {
	in := NewInterner()
	root := buildPointerAttrSet(in, 64)
	b.ResetTimer()
	b.ReportAllocs()
	var sink int64
	for i := 0; i < b.N; i++ {
		sink += walkPointer(root, 2)
	}
	_ = sink
}

func BenchmarkValueArenaWalk(b *testing.B) {
	in := NewInterner()
	var a arena
	root := buildArenaAttrSet(in, &a, 64)
	b.ResetTimer()
	b.ReportAllocs()
	var sink int64
	for i := 0; i < b.N; i++ {
		sink += walkArena(&a, root, 2)
	}
	_ = sink
}
