// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

func parseTOMLDocument(ev *Evaluator, src string) (*Value, error) {
	root := Attrs(false)
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			return nil, fmt.Errorf("line %d: nested TOML tables not supported", i+1)
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		v, err := parseTOMLScalar(val)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		root.Attrs.Binds[ev.Intern.Intern(key)] = &AttrBinding{Value: v}
	}
	return root, nil
}

func parseTOMLScalar(s string) (*Value, error) {
	if strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		unq, err := strconv.Unquote(s)
		if err != nil {
			return nil, err
		}
		return Str(unq, parser.Pos{}), nil
	}
	if s == "true" {
		return Bool(true), nil
	}
	if s == "false" {
		return Bool(false), nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return Int(n), nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &Value{Kind: VKFloat, Float: f}, nil
}
