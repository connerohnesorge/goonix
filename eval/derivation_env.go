// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"sort"
	"strconv"
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
	switch v.Kind {
	case VKString:
		if v.String != "" {
			c.addSrc(v.String)
		}
	case VKPath:
		if v.Path != "" {
			c.addSrc(v.Path)
		}
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

func (ev *Evaluator) isDerivationAttrSet(v *Value) (bool, error) {
	fv, err := ev.forceValue(v)
	if err != nil || fv == nil || fv.Kind != VKAttrs {
		return false, err
	}
	b := fv.Attrs.Binds[ev.Intern.Intern("type")]
	if b == nil {
		return false, nil
	}
	tv, err := ev.attrsBindingForce(b)
	if err != nil {
		return false, err
	}
	s, err := ev.forceString(tv)
	return s == "derivation", err
}

func (ev *Evaluator) coerceDerivationEnvString(v *Value, pos parser.Pos, col *derivationInputCollector) (string, error) {
	fv, err := ev.forceValue(v)
	if err != nil {
		return "", err
	}
	switch fv.Kind {
	case VKNull:
		return "", nil
	case VKBool:
		if fv.Bool {
		 return "1", nil
		}
		return "", nil
	case VKInt:
		return strconv.FormatInt(fv.Int, 10), nil
	case VKFloat:
		return strconv.FormatFloat(fv.Float, 'g', -1, 64), nil
	case VKString:
		col.mergeValueContext(fv)
		return fv.String, nil
	case VKPath:
		col.mergeValueContext(fv)
		return fv.Path, nil
	case VKList:
		var parts []string
		for _, el := range fv.List {
			subCol := newDerivationInputCollector()
			p, err := ev.coerceDerivationEnvString(el, pos, subCol)
			if err != nil {
				return "", err
			}
			if p != "" {
				parts = append(parts, p)
			}
			col.merge(subCol)
		}
		return strings.Join(parts, " "), nil
	case VKAttrs:
		ok, err := ev.isDerivationAttrSet(fv)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", evalErr(pos, "cannot coerce %s to a string", valueKindName(fv.Kind))
		}
		outName := "out"
		if b := fv.Attrs.Binds[ev.Intern.Intern("outputName")]; b != nil {
			if tv, err := ev.attrsBindingForce(b); err == nil {
				if s, err := ev.forceString(tv); err == nil && s != "" {
					outName = s
				}
			}
		}
		var outPath string
		if b := fv.Attrs.Binds[ev.Intern.Intern(outName)]; b != nil {
			if tv, err := ev.attrsBindingForce(b); err == nil {
				switch tv.Kind {
				case VKPath:
					outPath = tv.Path
				default:
					outPath, _ = ev.forceString(tv)
				}
			}
		}
		if outPath == "" {
			if b := fv.Attrs.Binds[ev.Intern.Intern("outPath")]; b != nil {
				if tv, err := ev.attrsBindingForce(b); err == nil && tv.Kind == VKPath {
					outPath = tv.Path
				}
			}
		}
		var drvPath string
		if b := fv.Attrs.Binds[ev.Intern.Intern("drvPath")]; b != nil {
			if tv, err := ev.attrsBindingForce(b); err == nil && tv.Kind == VKPath {
				drvPath = tv.Path
			}
		}
		if drvPath != "" {
			col.addDrv(drvPath, outName)
		}
		col.addSrc(outPath)
		return outPath, nil
	default:
		return "", evalErr(pos, "cannot coerce %s to a string", valueKindName(fv.Kind))
	}
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
		ignoreNulls, err = ev.forceBool(ev, tv, nil)
		if err != nil {
			return err
		}
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
	var outputHash, outputHashAlgo, outputHashMode string

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
				s, err := ev.coerceDerivationEnvString(el, pos, subCol)
				if err != nil {
					return err
				}
				drv.Args = append(drv.Args, s)
				col.merge(subCol)
			}
			continue
		}

		subCol := newDerivationInputCollector()
		s, err := ev.coerceDerivationEnvString(val, pos, subCol)
		if err != nil {
			return err
		}
		drv.Env[key] = s
		col.merge(subCol)

		switch key {
		case "builder":
			drv.Builder = s
		case "system":
			drv.System = s
		case "outputHash":
			outputHash = s
		case "outputHashAlgo":
			outputHashAlgo = s
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
	_ = outputHashAlgo
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
	return nil
}
