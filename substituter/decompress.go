// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package substituter

import (
	"compress/bzip2"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// decompressReader wraps r with the decompressor a narinfo Compression field
// names, so a NAR streams from the network without being held in memory.
func decompressReader(r io.Reader, compression string) (io.ReadCloser, error) {
	switch compression {
	case "", "none":
		return io.NopCloser(r), nil
	case "xz":
		xr, err := xz.NewReader(r)
		if err != nil {
			return nil, err
		}
		return io.NopCloser(xr), nil
	case "zstd":
		dec, err := zstd.NewReader(r)
		if err != nil {
			return nil, err
		}
		return dec.IOReadCloser(), nil
	case "bzip2":
		return io.NopCloser(bzip2.NewReader(r)), nil
	default:
		return nil, fmt.Errorf("unknown compression %q", compression)
	}
}
