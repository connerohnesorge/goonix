// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"strings"

	"github.com/connerohnesorge/goonix/store"
)

// verifyDerivationEnvContext checks Nix derivationStrict invariants: every
// derivation-output string context element must reference a drv in inputDrvs,
// and the coerced env text must contain that output's store path.
func (ev *Evaluator) verifyDerivationEnvContext(drv *DerivationValue, envCtx map[string][]StringContextElem) error {
	if drv == nil {
		return nil
	}
	inputSet := make(map[string]struct{}, len(drv.InputDerivations))
	for p := range drv.InputDerivations {
		inputSet[p] = struct{}{}
	}
	for key, s := range drv.Env {
		ctx := envCtx[key]
		for _, elem := range ctx {
			if elem.DrvPath == "" {
				continue
			}
			out := elem.Output
			if out == "" {
				out = "out"
			}
			if _, ok := inputSet[elem.DrvPath]; !ok {
				return fmt.Errorf("derivation %q env[%q]: string context references %q but that drv is not in inputDrvs", drv.Name, key, elem.DrvPath)
			}
			want, ok := ev.outputStorePath(elem.DrvPath, out)
			if !ok {
				return fmt.Errorf("derivation %q env[%q]: string context references unknown output %q of %q", drv.Name, key, out, elem.DrvPath)
			}
			if !strings.Contains(s, want) {
				return fmt.Errorf("derivation %q env[%q]=%q does not contain output path %q from context drv %q", drv.Name, key, truncEnv(s), want, elem.DrvPath)
			}
			if len(ctx) == 1 && elem.StorePath == "" && isSingleStorePathRef(s) {
				if s != want && !strings.HasPrefix(s, want+"/") {
					return fmt.Errorf("derivation %q env[%q]=%q inconsistent with string context drv %q output %q", drv.Name, key, s, elem.DrvPath, want)
				}
			}
		}
	}
	return nil
}

func isSingleStorePathRef(s string) bool {
	if !strings.HasPrefix(s, store.DefaultStoreDir+"/") {
		return false
	}
	return !strings.ContainsAny(s, " \t\n")
}

func truncEnv(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:120] + "..."
}

func (ev *Evaluator) outputStorePath(drvPath, output string) (string, bool) {
	if ev == nil || drvPath == "" {
		return "", false
	}
	if ev.drvStrictByPath != nil {
		if d, ok := ev.drvStrictByPath[drvPath]; ok && d != nil {
			if d.OutPaths != nil {
				if p, ok := d.OutPaths[output]; ok {
					return p, true
				}
			}
			if p, ok := d.Env[output]; ok {
				return p, true
			}
		}
	}
	if d, ok := ev.RegisteredDerivation(drvPath); ok && d != nil {
		if p, ok := d.Env[output]; ok {
			return p, true
		}
		if o, ok := d.Outputs[output]; ok && o != nil {
			return o.Path, true
		}
	}
	return "", false
}

func (ev *Evaluator) verifyStructuredDerivationEnvContext(drv *DerivationValue, col *derivationInputCollector) error {
	if drv == nil || col == nil {
		return nil
	}
	jsonText := drv.Env["__json"]
	for p, outs := range col.drvs {
		for out := range outs {
			want, ok := ev.outputStorePath(p, out)
			if !ok {
				return fmt.Errorf("derivation %q structured attrs: input drv %q output %q not in store cache", drv.Name, p, out)
			}
			if !strings.Contains(jsonText, want) {
				return fmt.Errorf("derivation %q __json does not contain output path %q from input drv %q", drv.Name, want, p)
			}
		}
	}
	return nil
}
