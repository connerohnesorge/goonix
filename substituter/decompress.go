// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package substituter

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

func decompress(data []byte, compression string) ([]byte, error) {
	switch compression {
	case "", "none":
		return data, nil
	case "xz":
		r, err := xz.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return io.ReadAll(r)
	case "zstd":
		dec, err := zstd.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return io.ReadAll(dec)
	case "bzip2":
		return nil, fmt.Errorf("compression %q not supported (pure Go)", compression)
	default:
		return nil, fmt.Errorf("unknown compression %q", compression)
	}
}
