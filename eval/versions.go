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
	"strconv"
	"unicode"
)

// compareVersions compares Nix-style version strings (same semantics as Nix).
func compareVersions(v1, v2 string) int {
	p1 := 0
	p2 := 0
	for p1 < len(v1) || p2 < len(v2) {
		c1 := nextVersionComponent(v1, &p1)
		c2 := nextVersionComponent(v2, &p2)
		if componentsLT(c1, c2) {
			return -1
		}
		if componentsLT(c2, c1) {
			return 1
		}
	}
	return 0
}

func nextVersionComponent(s string, p *int) string {
	for *p < len(s) && (s[*p] == '.' || s[*p] == '-') {
		*p++
	}
	if *p >= len(s) {
		return ""
	}
	start := *p
	if unicode.IsDigit(rune(s[*p])) {
		for *p < len(s) && unicode.IsDigit(rune(s[*p])) {
			*p++
		}
	} else {
		for *p < len(s) && !unicode.IsDigit(rune(s[*p])) && s[*p] != '.' && s[*p] != '-' {
			*p++
		}
	}
	return s[start:*p]
}

func componentsLT(c1, c2 string) bool {
	n1, err1 := strconv.Atoi(c1)
	n2, err2 := strconv.Atoi(c2)
	if err1 == nil && err2 == nil {
		return n1 < n2
	}
	if c1 == "" && err2 == nil {
		return true
	}
	if c1 == "pre" && c2 != "pre" {
		return true
	}
	if c2 == "pre" {
		return false
	}
	if err2 == nil {
		return true
	}
	if err1 == nil {
		return false
	}
	return c1 < c2
}
