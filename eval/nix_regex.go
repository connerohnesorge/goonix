// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

func validateNixRegex(reStr string) error {
	if strings.Contains(reStr, "(?i)") || strings.Contains(reStr, "(?I)") {
		return fmt.Errorf("invalid regular expression %q", reStr)
	}
	for i := 0; i < len(reStr); i++ {
		if reStr[i] != '\\' {
			continue
		}
		if i+1 >= len(reStr) {
			break
		}
		switch reStr[i+1] {
		case 'd', 'D', 'b', 'B':
			return fmt.Errorf("invalid regular expression %q", reStr)
		}
	}
	return nil
}

func compileNixRegex(reStr string) (*regexp.Regexp, error) {
	if err := validateNixRegex(reStr); err != nil {
		return nil, err
	}
	re, err := regexp.Compile("(?s:" + reStr + ")")
	if err != nil {
		return nil, fmt.Errorf("invalid regular expression %q", reStr)
	}
	re.Longest()
	return re, nil
}

// nixRegexMatch implements builtins.match: full-string POSIX ERE match; null if no match,
// otherwise a list of capture groups (empty list when there are none). Unmatched groups are null.
func nixRegexMatch(reStr, s string) ([]*string, bool, error) {
	re, err := compileNixRegex(reStr)
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
		return []*string{}, true, nil
	}
	out := make([]*string, n)
	for i := 0; i < n; i++ {
		start, end := loc[2+i*2], loc[3+i*2]
		if start >= 0 && end >= 0 {
			v := s[start:end]
			out[i] = &v
		}
	}
	return out, true, nil
}

// nixRegexSplit implements builtins.split: split a string by a POSIX ERE, returning a list
// of alternating strings and lists (empty list when the separator has no capture groups).
func nixRegexSplit(reStr, s string) ([]any, error) {
	re, err := compileNixRegex(reStr)
	if err != nil {
		return nil, err
	}
	nCap := re.NumSubexp()
	var out []any
	textStart := 0
	pos := 0
	for pos <= len(s) {
		loc := re.FindStringSubmatchIndex(s[pos:])
		if loc != nil && loc[0] == 0 {
			out = append(out, s[textStart:pos])
			if nCap == 0 {
				out = append(out, []string{})
			} else {
				caps := make([]string, nCap)
				for i := 0; i < nCap; i++ {
					start, end := loc[2+i*2], loc[3+i*2]
					if start >= 0 && end >= 0 {
						caps[i] = s[pos+start : pos+end]
					}
				}
				out = append(out, caps)
			}
			adv := loc[1]
			if adv == 0 {
				if pos < len(s) && pos > 0 {
					pos++
					continue
				}
				pos++
				if pos >= len(s) {
					textStart = pos
					out = append(out, "")
				}
				continue
			}
			pos += adv
			textStart = pos
			continue
		}
		if pos >= len(s) {
			break
		}
		pos++
	}
	if textStart < len(s) {
		out = append(out, s[textStart:])
	} else if textStart == len(s) && len(out) > 0 {
		out = append(out, "")
	}
	if len(out) == 0 {
		return []any{s}, nil
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

func matchGroupsToValues(ev *Evaluator, groups []*string, pos parser.Pos) (*Value, error) {
	elems := make([]*Value, len(groups))
	for i, g := range groups {
		if g == nil {
			elems[i] = Null()
		} else {
			elems[i] = Str(*g, pos)
		}
	}
	return List(elems...), nil
}
