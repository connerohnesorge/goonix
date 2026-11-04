// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package substituter

import (
	"bytes"
	"testing"
)

func TestDecompressNonePassthrough(t *testing.T) {
	in := []byte("hello nar")
	out, err := decompress(in, "none")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("got %q", out)
	}
}
