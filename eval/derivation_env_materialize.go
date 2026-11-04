// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"strings"

	"github.com/connerohnesorge/goonix/store"
)

const placeholderText = "unknown-tee"

func (ev *Evaluator) materializeDerivationEnvPlaceholders(drv *DerivationValue, envCtx map[string][]StringContextElem) {
	if drv == nil || len(envCtx) == 0 {
		return
	}
	for key, ctx := range envCtx {
		if len(ctx) == 0 {
			continue
		}
		s, ok := drv.Env[key]
		if !ok {
			continue
		}
		if mat, err := ev.materializePlaceholderString(s, ctx, drv); err == nil && mat != s {
			drv.Env[key] = mat
		}
	}
}

func (ev *Evaluator) materializePlaceholderString(s string, ctx []StringContextElem, self *DerivationValue) (string, error) {
	if len(ctx) == 0 {
		return s, nil
	}
	var out strings.Builder
	rest := s
	for _, elem := range ctx {
		i := strings.Index(rest, placeholderText)
		if i < 0 {
			break
		}
		out.WriteString(rest[:i])
		p, err := ev.resolveStringContextElem(elem, self)
		if err != nil {
			return "", err
		}
		out.WriteString(p)
		rest = rest[i+len(placeholderText):]
	}
	out.WriteString(rest)
	return out.String(), nil
}

func (ev *Evaluator) resolveStringContextElem(elem StringContextElem, self *DerivationValue) (string, error) {
	outName := elem.Output
	if outName == "" {
		outName = "out"
	}
	if elem.DrvPath == "" {
		if self != nil && self.OutPaths != nil {
			if p, ok := self.OutPaths[outName]; ok {
				return derivationEnvPlaceholderRef(p), nil
			}
		}
		return "", fmt.Errorf("placeholder output %q not available", outName)
	}
	if elem.StorePath != "" && elem.DrvPath == "" {
		return elem.StorePath, nil
	}
	p, ok := ev.outputStorePath(elem.DrvPath, outName)
	if !ok {
		return "", fmt.Errorf("unknown output %q of %q", outName, elem.DrvPath)
	}
	return p, nil
}

// derivationEnvPlaceholderRef is how Nix prints builtins.placeholder in derivation env strings.
func derivationEnvPlaceholderRef(storePath string) string {
	base := strings.TrimPrefix(storePath, store.DefaultStoreDir+"/")
	if i := strings.Index(base, "-"); i > 0 {
		return "/" + base[:i]
	}
	return storePath
}
