// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package substituter

import (
	"bytes"
	"io"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestDecompressReader(t *testing.T) {
	in := []byte("hello nar")
	var zbuf bytes.Buffer
	zw, err := zstd.NewWriter(&zbuf)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = zw.Write(in)
	_ = zw.Close()
	// bzip2 of "hello nar", made with `printf 'hello nar' | bzip2`.
	bz := []byte{0x42, 0x5a, 0x68, 0x39, 0x31, 0x41, 0x59, 0x26, 0x53, 0x59, 0x29, 0x03, 0x6d, 0x15, 0x00, 0x00, 0x01, 0x91, 0x80, 0x40, 0x00, 0x22, 0x45, 0x90, 0x00, 0x20, 0x00, 0x31, 0x0c, 0x08, 0x20, 0x0f, 0x28, 0x1c, 0x89, 0x4f, 0x8b, 0xb9, 0x22, 0x9c, 0x28, 0x48, 0x14, 0x81, 0xb6, 0x8a, 0x80}
	for _, c := range []struct {
		name string
		data []byte
	}{{"none", in}, {"zstd", zbuf.Bytes()}, {"bzip2", bz}} {
		r, err := decompressReader(bytes.NewReader(c.data), c.name)
		if err != nil {
			t.Fatal(err)
		}
		out, err := io.ReadAll(r)
		r.Close()
		if err != nil || !bytes.Equal(out, in) {
			t.Fatalf("%s: got %q, %v", c.name, out, err)
		}
	}
	if _, err := decompressReader(bytes.NewReader(in), "lz4"); err == nil {
		t.Fatal("unknown compression accepted")
	}
}
