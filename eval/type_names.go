// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import "fmt"

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
	case VKDeferred:
		return "thunk"
	default:
		return fmt.Sprintf("unknown(%d)", k)
	}
}

// nixShowType is Nix's showType: the type as it appears in error messages.
func nixShowType(v *Value) string {
	switch v.Kind {
	case VKNull:
		return "null"
	case VKBool:
		return "a Boolean"
	case VKInt:
		return "an integer"
	case VKFloat:
		return "a float"
	case VKString:
		return "a string"
	case VKPath:
		return "a path"
	case VKList:
		return "a list"
	case VKAttrs:
		return "a set"
	case VKBuiltin:
		return "a built-in function"
	case VKPartialBuiltin:
		return "a partially applied built-in function"
	case VKLambda:
		return "a function"
	default:
		return valueKindName(v.Kind)
	}
}
