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

package eval

import (
	"bytes"
	"encoding/json"
)

// marshalNixJSON encodes v the way Nix writes JSON (builtins.toJSON and
// __json): no HTML escaping of <, > and &, and U+2028/U+2029 written raw.
// encoding/json escapes all of these by default, which changes .drv bytes.
func marshalNixJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	b := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	if !bytes.Contains(b, []byte(`\u202`)) {
		return b, nil
	}
	// Every backslash in encoder output starts an escape, so walking escape
	// by escape cannot mistake an escaped backslash for one.
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		if b[i] != '\\' || i+1 >= len(b) {
			out = append(out, b[i])
			continue
		}
		if b[i+1] == 'u' && i+5 < len(b) {
			switch string(b[i+2 : i+6]) {
			case "2028":
				out = append(out, "\u2028"...)
				i += 5
				continue
			case "2029":
				out = append(out, "\u2029"...)
				i += 5
				continue
			}
		}
		out = append(out, b[i], b[i+1])
		i++
	}
	return out, nil
}
