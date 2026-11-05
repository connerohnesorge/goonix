// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"regexp"

	"github.com/connerohnesorge/goonix/parser"
)

// nixRegexMatch implements builtins.match: full-string POSIX ERE match; null if no match,
// otherwise a list of capture groups (empty list when there are none).
func nixRegexMatch(reStr, s string) ([]string, bool, error) {
	re, err := regexp.Compile(reStr)
	if err != nil {
		return nil, false, err
	}
	loc := re.FindStringSubmatchIndex(s)
	if loc == nil {
		return nil, false, nil
	}
	if loc[0] != 0 || loc[1] != len(s) {
		return nil, false, nil
	}
	n := re.NumSubexp()
	if n == 0 {
		return []string{}, true, nil
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		start, end := loc[2+i*2], loc[3+i*2]
		if start >= 0 && end >= 0 {
			out[i] = s[start:end]
		}
	}
	return out, true, nil
}

// nixRegexSplit implements builtins.split: split a string by a POSIX ERE, returning a list
// of alternating strings and lists (empty list when the separator has no capture groups).
func nixRegexSplit(reStr, s string) ([]any, error) {
	re, err := regexp.Compile(reStr)
	if err != nil {
		return nil, err
	}
	idx := re.FindAllStringSubmatchIndex(s, -1)
	if len(idx) == 0 {
		return []any{s}, nil
	}
	nCap := re.NumSubexp()
	var out []any
	pos := 0
	for _, loc := range idx {
		if loc[0] < pos {
			continue
		}
		out = append(out, s[pos:loc[0]])
		if nCap == 0 {
			out = append(out, []string{})
		} else {
			caps := make([]string, nCap)
			for i := 0; i < nCap; i++ {
				start, end := loc[2+i*2], loc[3+i*2]
				if start >= 0 && end >= 0 {
					caps[i] = s[start:end]
				}
			}
			out = append(out, caps)
		}
		pos = loc[1]
	}
	if pos < len(s) {
		out = append(out, s[pos:])
	}
	if len(idx) > 0 && idx[len(idx)-1][1] == len(s) {
		out = append(out, "")
	}
	return out, nil
}

func nixSplitToValues(ev *Evaluator, parts []any, pos parser.Pos) (*Value, error) {
	elems := make([]*Value, len(parts))
	for i, p := range parts {
		switch x := p.(type) {
		case string:
			elems[i] = Str(x, pos)
		case []string:
			sub := make([]*Value, len(x))
			for j, g := range x {
				sub[j] = Str(g, pos)
			}
			elems[i] = List(sub...)
		default:
			return nil, fmt.Errorf("internal split part %T", p)
		}
	}
	return List(elems...), nil
}
