// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
)

func (ev *Evaluator) structuredAttrsEnabled(attrs *Value) (bool, error) {
	b := attrs.Attrs.Binds[ev.Intern.Intern("__structuredAttrs")]
	if b == nil {
		return false, nil
	}
	tv, err := ev.attrsBindingForce(b)
	if err != nil {
		return false, err
	}
	tv, err = ev.forceValue(tv)
	if err != nil {
		return false, err
	}
	return ev.forceBool(ev, tv, nil)
}

func (ev *Evaluator) fillStructuredDerivationFromAttrs(attrs *Value, drv *DerivationValue, pos parser.Pos, ignoreNulls bool) error {
	syms := make([]data.Symbol, 0, len(attrs.Attrs.Binds))
	for sym := range attrs.Attrs.Binds {
		syms = append(syms, sym)
	}
	sort.Slice(syms, func(i, j int) bool {
		return ev.Intern.Lookup(syms[i]) < ev.Intern.Lookup(syms[j])
	})

	col := newDerivationInputCollector()
	drv.Env = make(map[string]string)
	structured := make(map[string]any)

	for _, sym := range syms {
		key := ev.Intern.Lookup(sym)
		if key == "__ignoreNulls" || key == "__structuredAttrs" {
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

		switch key {
		case "args":
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
				s, _, err := ev.coerceDerivationEnvString(el, pos, subCol)
				if err != nil {
					return err
				}
				drv.Args = append(drv.Args, s)
				col.merge(subCol)
			}
			continue
		case "builder":
			subCol := newDerivationInputCollector()
			s, _, err := ev.coerceDerivationEnvString(val, pos, subCol)
			if err != nil {
				return err
			}
			drv.Builder = s
			col.merge(subCol)
		case "system":
			subCol := newDerivationInputCollector()
			s, _, err := ev.coerceDerivationEnvString(val, pos, subCol)
			if err != nil {
				return err
			}
			drv.System = s
			col.merge(subCol)
		case "outputHashMode":
			s, err := ev.forceString(val)
			if err != nil {
				return err
			}
			if err := ev.checkOutputHashMode(pos, s); err != nil {
				return err
			}
		case "outputs":
			lv, err := ev.forceValue(val)
			if err != nil {
				return err
			}
			if lv.Kind != VKList {
				return evalErr(pos, "outputs must be a list")
			}
			outs := make(map[string]struct{})
			for _, el := range lv.List {
				s, err := ev.forceString(el)
				if err != nil {
					return err
				}
				if _, dup := outs[s]; dup {
					return evalErr(pos, "duplicate derivation output '%s'", s)
				}
				if s == "drvPath" {
					return evalErr(pos, "invalid derivation output name 'drvPath'")
				}
				outs[s] = struct{}{}
			}
			if len(outs) == 0 {
				return evalErr(pos, "derivation cannot have an empty set of outputs")
			}
			drv.Outputs = outs
		case "allowedReferences", "allowedRequisites", "disallowedReferences", "disallowedRequisites", "maxSize", "maxClosureSize":
			fmt.Fprintf(os.Stderr, "warning: In a derivation named '%s', 'structuredAttrs' disables the effect of the derivation attribute '%s'; use 'outputChecks.<output>.%s' instead\n", drv.Name, key, key)
		}

		jv, err := ev.valueToStructuredJSON(val, pos, col)
		if err != nil {
			return err
		}
		structured[key] = jv
	}

	raw, err := json.Marshal(structured)
	if err != nil {
		return err
	}
	drv.Env["__json"] = string(raw)

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
	if err := ev.verifyStructuredDerivationEnvContext(drv, col); err != nil {
		return err
	}
	return nil
}

func (ev *Evaluator) valueToStructuredJSON(v *Value, pos parser.Pos, col *derivationInputCollector) (any, error) {
	fv, err := ev.mustWHNF(v, pos)
	if err != nil {
		return nil, err
	}
	switch fv.Kind {
	case VKNull:
		return nil, nil
	case VKBool:
		return fv.Bool, nil
	case VKInt:
		return fv.Int, nil
	case VKFloat:
		if nixFloatNonFinite(fv.Float) {
			return nil, nil
		}
		return json.Number(formatNixJSONFloat(fv.Float)), nil
	case VKString:
		col.mergeValueContext(fv)
		return fv.String, nil
	case VKPath:
		sv, err := ev.coerceToString(fv, CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: true})
		if err != nil {
			return nil, err
		}
		col.mergeValueContext(sv)
		return sv.String, nil
	case VKList:
		out := make([]any, len(fv.List))
		for i, el := range fv.List {
			x, err := ev.valueToStructuredJSON(el, pos, col)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case VKAttrs:
		if ev.isDerivationAttrSet(fv) {
			subCol := newDerivationInputCollector()
			s, err := ev.derivationJSONPath(fv, pos, subCol)
			if err != nil {
				return nil, err
			}
			col.merge(subCol)
			return s, nil
		}
		m := make(map[string]any)
		syms := make([]data.Symbol, 0, len(fv.Attrs.Binds))
		for sym := range fv.Attrs.Binds {
			syms = append(syms, sym)
		}
		sort.Slice(syms, func(i, j int) bool {
			return ev.Intern.Lookup(syms[i]) < ev.Intern.Lookup(syms[j])
		})
		for _, sym := range syms {
			name := ev.Intern.Lookup(sym)
			b := fv.Attrs.Binds[sym]
			el, err := ev.attrBindingValue(b)
			if err != nil {
				return nil, err
			}
			x, err := ev.valueToStructuredJSON(el, pos, col)
			if err != nil {
				return nil, err
			}
			m[name] = x
		}
		return m, nil
	case VKLambda, VKBuiltin:
		return nil, nil
	default:
		return nil, nil
	}
}

func (ev *Evaluator) isDerivationAttrSet(v *Value) bool {
	if v == nil || v.Kind != VKAttrs {
		return false
	}
	b := v.Attrs.Binds[ev.Intern.Intern("type")]
	if b == nil {
		return false
	}
	tv, err := ev.attrBindingValue(b)
	if err != nil {
		return false
	}
	ts, err := ev.forceString(tv)
	return err == nil && ts == "derivation"
}

func (ev *Evaluator) derivationJSONPath(v *Value, pos parser.Pos, col *derivationInputCollector) (string, error) {
	for _, name := range []string{"outPath", "out"} {
		b := v.Attrs.Binds[ev.Intern.Intern(name)]
		if b == nil {
			continue
		}
		el, err := ev.attrBindingValue(b)
		if err != nil {
			return "", err
		}
		s, _, err := ev.coerceDerivationEnvString(el, pos, col)
		return s, err
	}
	return "", evalErr(pos, "derivation missing outPath")
}
