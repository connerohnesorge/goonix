// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"math"
	"strconv"
	"strings"
)

func nixFloatNonFinite(f float64) bool {
	return math.IsNaN(f) || math.IsInf(f, 0)
}

// formatNixToStringFloat is std::to_string(double): six digits after the
// decimal, with inf and nan spelled the way libstdc++ prints them.
func formatNixToStringFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		if math.Signbit(f) {
			return "-nan"
		}
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	default:
		return strconv.FormatFloat(f, 'f', 6, 64)
	}
}

// formatNixJSONFloat matches nlohmann's dump of a finite double: the shortest
// round-trip, scientific notation when the exponent is below -4 or at least
// 15, and a trailing ".0" on values that would otherwise look like integers.
func formatNixJSONFloat(f float64) string {
	sci := strconv.FormatFloat(f, 'e', -1, 64)
	if exp := scientificExponent(sci); exp < -4 || exp >= 15 {
		return sci
	}
	fixed := strconv.FormatFloat(f, 'f', -1, 64)
	if !strings.Contains(fixed, ".") {
		return fixed + ".0"
	}
	return fixed
}

func scientificExponent(sci string) int {
	i := strings.LastIndexAny(sci, "eE")
	if i < 0 || i+1 >= len(sci) {
		return 0
	}
	n, err := strconv.Atoi(sci[i+1:])
	if err != nil {
		return 0
	}
	return n
}
