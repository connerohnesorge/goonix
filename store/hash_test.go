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

package store_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/store"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

func TestParseHashBase32(t *testing.T) {
	h, err := store.ParseHash("sha256:0cvn4pmxibandjzsqanpxdmp7z5czdzlqwdq8ddql8sm1n4kfqmr")
	if err != nil {
		t.Fatal(err)
	}
	if h.Algo() != nixhash.SHA256 {
		t.Fatalf("algo = %v, want sha256", h.Algo())
	}
}

func TestRecursivePathHash(t *testing.T) {
	dir := filepath.Join("testdata", "nar")
	got, err := store.RecursivePathHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty hash")
	}
}

func TestDumpPath(t *testing.T) {
	var buf []byte
	w := &bytesWriter{buf: &buf}
	if err := store.DumpPath(w, filepath.Join("testdata", "nar")); err != nil {
		t.Fatal(err)
	}
	if len(buf) == 0 {
		t.Fatal("expected non-empty NAR output")
	}
}

type bytesWriter struct {
	buf *[]byte
}

func (b *bytesWriter) Write(p []byte) (int, error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

func TestCompressHash(t *testing.T) {
	out := store.CompressHash([]byte{1, 2, 3, 4, 5}, 4)
	if len(out) != 4 {
		t.Fatalf("len = %d, want 4", len(out))
	}
}
