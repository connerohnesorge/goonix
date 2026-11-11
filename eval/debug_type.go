// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"os"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

func debugTypeMismatchEnabled() bool {
	v := os.Getenv("GOONIX_DEBUG_TYPE")
	return v != "" && v != "0" && !strings.EqualFold(v, "false")
}

func valueKindName(k ValueKind) string {
	switch k {
	case VKNull:
		return "null"
	case VKBool:
		return "bool"
	case VKInt:
		return "int"
	case VKFloat:
		return "float"
	case VKString:
		return "string"
	case VKPath:
		return "path"
	case VKList:
		return "list"
	case VKAttrs:
		return "set"
	case VKLambda:
		return "lambda"
	case VKBuiltin:
		return "builtin"
	case VKPartialBuiltin:
		return "partial-builtin"
	case VKDerivation:
		return "derivation"
	case VKDeferred:
		return "thunk"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

func (ev *Evaluator) describeValueForDebug(v *Value) string {
	if v == nil {
		return "nil"
	}
	var b strings.Builder
	b.WriteString("kind=")
	b.WriteString(valueKindName(v.Kind))
	if v.Kind == VKDeferred && v.Deferred != nil {
		t := v.Deferred
		b.WriteString(fmt.Sprintf(" locked=%v memo=%v", t.locked, t.memo != nil))
		if t.pos.File != "" {
			b.WriteString(fmt.Sprintf(" thunkPos=%s:%d:%d", t.pos.File, t.pos.Line, t.pos.Col))
		}
	}
	return b.String()
}

// logTypeMismatchWant logs when GOONIX_DEBUG_TYPE is set and a type coercion failed.
func (ev *Evaluator) logTypeMismatchWant(pos parser.Pos, v *Value, want string) {
	if !debugTypeMismatchEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "goonix type mismatch at %s:%d:%d: want %s; got %s\n",
		pos.File, pos.Line, pos.Col, want, ev.describeValueForDebug(v))
}
