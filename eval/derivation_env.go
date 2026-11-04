// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

type derivationInputCollector struct {
	drvs map[string]map[string]struct{}
	srcs map[string]struct{}
}

func newDerivationInputCollector() *derivationInputCollector {
	return &derivationInputCollector{
		drvs: make(map[string]map[string]struct{}),
		srcs: make(map[string]struct{}),
	}
}

func (c *derivationInputCollector) addDrv(drvPath, output string) {
	if drvPath == "" {
		return
	}
	m, ok := c.drvs[drvPath]
	if !ok {
		m = make(map[string]struct{})
		c.drvs[drvPath] = m
	}
	m[output] = struct{}{}
}

func (c *derivationInputCollector) addSrc(path string) {
	if path == "" || !strings.HasPrefix(path, store.DefaultStoreDir+"/") {
		return
	}
	c.srcs[path] = struct{}{}
}

func (c *derivationInputCollector) mergeValueContext(v *Value) {
	if v == nil {
		return
	}
	for _, elem := range v.Ctx {
		if elem.DrvPath != "" {
			out := elem.Output
			if out == "" {
				out = "out"
			}
			c.addDrv(elem.DrvPath, out)
		}
		if elem.StorePath != "" {
			c.addSrc(elem.StorePath)
		}
	}
}

func (c *derivationInputCollector) merge(other *derivationInputCollector) {
	if other == nil {
		return
	}
	for p, outs := range other.drvs {
		for o := range outs {
			c.addDrv(p, o)
		}
	}
	for p := range other.srcs {
		c.addSrc(p)
	}
}

func (c *derivationInputCollector) toGoNixMaps() (map[string][]string, []string) {
	drvs := make(map[string][]string, len(c.drvs))
	for p, outs := range c.drvs {
		names := make([]string, 0, len(outs))
		for o := range outs {
			names = append(names, o)
		}
		sort.Strings(names)
		drvs[p] = names
	}
	srcs := make([]string, 0, len(c.srcs))
	for p := range c.srcs {
		srcs = append(srcs, p)
	}
	sort.Strings(srcs)
	return drvs, srcs
}

func isSRIHashString(h string) bool {
	if h == "" {
		return false
	}
	if strings.HasPrefix(h, "sha256-") || strings.HasPrefix(h, "sha512-") ||
		strings.HasPrefix(h, "sha1-") || strings.HasPrefix(h, "md5-") {
		return true
	}
	return false
}

func (ev *Evaluator) coerceDerivationEnvString(v *Value, pos parser.Pos, col *derivationInputCollector) (string, []StringContextElem, error) {
	prev := ev.materializeStorePaths
	ev.materializeStorePaths = true
	defer func() { ev.materializeStorePaths = prev }()
	sv, err := ev.coerceToString(v, CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: true})
	if err != nil {
		return "", nil, err
	}
	col.mergeValueContext(sv)
	return sv.String, copyStringContext(sv.Ctx), nil
}

func parseOutputNamesFromEnv(s string) (map[string]struct{}, error) {
	out := make(map[string]struct{})
	for _, part := range strings.Fields(s) {
		if part == "drvPath" {
			return nil, evalErr(parser.Pos{}, "invalid derivation output name 'drvPath'")
		}
		out[part] = struct{}{}
	}
	if len(out) == 0 {
		return nil, evalErr(parser.Pos{}, "derivation cannot have an empty set of outputs")
	}
	return out, nil
}

func (ev *Evaluator) fillDerivationFromAttrs(attrs *Value, drv *DerivationValue, pos parser.Pos) error {
	if attrs == nil || attrs.Kind != VKAttrs {
		return evalErr(pos, "derivationStrict expects attribute set")
	}

	ignoreNulls := false
	if b := attrs.Attrs.Binds[ev.Intern.Intern("__ignoreNulls")]; b != nil {
		tv, err := ev.attrsBindingForce(b)
		if err != nil {
			return err
		}
		tv, err = ev.forceValue(tv)
		if err != nil {
			return err
		}
		ignoreNulls, err = ev.forceBool(ev, tv, nil)
		if err != nil {
			return err
		}
	}

	if structured, err := ev.structuredAttrsEnabled(attrs); err != nil {
		return err
	} else if structured {
		return ev.fillStructuredDerivationFromAttrs(attrs, drv, pos, ignoreNulls)
	}

	syms := make([]data.Symbol, 0, len(attrs.Attrs.Binds))
	for sym := range attrs.Attrs.Binds {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		return ev.Intern.Lookup(syms[i]) < ev.Intern.Lookup(syms[j])
	})

	col := newDerivationInputCollector()
	drv.Env = make(map[string]string)
	envCtx := make(map[string][]StringContextElem)
	var outputHash, outputHashMode string

	for _, sym := range syms {
		key := ev.Intern.Lookup(sym)
		if key == "__ignoreNulls" {
			continue
		}
		b := attrs.Attrs.Binds[sym]
		val, err := ev.attrsBindingForce(b)
		if err != nil {
			return err
		}
		val, err = ev.forceValue(val)
		if err != nil {
			return err
		}
		if ignoreNulls && val.Kind == VKNull {
			continue
		}

		if key == "args" {
			lv, err := ev.forceValue(val)
			if err != nil {
				return err
			}
			if lv.Kind != VKList {
				return evalErr(pos, "args must be a list")
			}
			drv.Args = nil
			for _, el := range lv.List {
				subCol := newDerivationInputCollector()
				s, elCtx, err := ev.coerceDerivationEnvString(el, pos, subCol)
				if err != nil {
					return err
				}
				drv.Args = append(drv.Args, s)
				col.merge(subCol)
				_ = elCtx
			}
			continue
		}

		subCol := newDerivationInputCollector()
		s, sCtx, err := ev.coerceDerivationEnvString(val, pos, subCol)
		if err != nil {
			return err
		}
		drv.Env[key] = s
		envCtx[key] = sCtx
		col.merge(subCol)

		switch key {
		case "builder":
			drv.Builder = s
		case "system":
			drv.System = s
		case "outputHash":
			outputHash = s
		case "outputHashAlgo":
			// stored in drv.Env[key] above
		case "outputHashMode":
			outputHashMode = s
		case "outputs":
			outs, err := parseOutputNamesFromEnv(s)
			if err != nil {
				return err
			}
			drv.Outputs = outs
		}
	}

	_ = outputHash
	_ = outputHashMode

	if drv.Builder == "" {
		return evalErr(pos, "required attribute 'builder' missing")
	}
	if drv.System == "" {
		return evalErr(pos, "required attribute 'system' missing")
	}
	if len(drv.Outputs) == 0 {
		drv.Outputs = map[string]struct{}{"out": {}}
	}

	drv.InputDerivations, drv.InputSources = col.toGoNixMaps()
	if err := ev.verifyDerivationEnvContext(drv, envCtx); err != nil {
		return err
	}
	return nil
}
