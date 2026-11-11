// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"github.com/connerohnesorge/goonix/parser"
)

// cloneAttrSetShallow copies attribute bindings without forcing values (shares thunks).
func cloneAttrSetShallow(v *Value) *Value {
	if v == nil || v.Kind != VKAttrs {
		return Attrs(false)
	}
	out := Attrs(v.Attrs.Recursive)
	for sym, bind := range v.Attrs.Binds {
		out.Attrs.Binds[sym] = bind
	}
	return out
}

func (ev *Evaluator) derivationOutputNames(drvAttrs *Value, pos parser.Pos) ([]string, error) {
	if drvAttrs == nil || drvAttrs.Kind != VKAttrs {
		return []string{"out"}, nil
	}
	sym := ev.Intern.Intern("outputs")
	b, ok := drvAttrs.Attrs.Binds[sym]
	if !ok {
		return []string{"out"}, nil
	}
	var lv *Value
	var err error
	if b.Thunk != nil {
		lv, err = ev.forceThunk(b.Thunk)
	} else {
		lv = b.Value
	}
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, lv)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(list))
	for _, el := range list {
		s, err := ev.forceString(el)
		if err != nil {
			return nil, err
		}
		names = append(names, s)
	}
	if len(names) == 0 {
		return []string{"out"}, nil
	}
	return names, nil
}

func (ev *Evaluator) lazyStrictDerivation(drvAttrs *Value, pos parser.Pos) *Value {
	strictThunk := &Thunk{
		eval: ev,
		pos:  pos,
		lazy: func(e *Evaluator) (*Value, error) {
			return builtinDerivationStrict(e, pos, []*Value{drvAttrs})
		},
	}
	return Deferred(strictThunk)
}

func (ev *Evaluator) lazySelectStrictAttr(strict *Value, attr string, pos parser.Pos) *Value {
	sym := ev.Intern.Intern(attr)
	t := &Thunk{
		eval: ev,
		pos:  pos,
		lazy: func(e *Evaluator) (*Value, error) {
			sv, err := e.forceValue(strict)
			if err != nil {
				return nil, err
			}
			if sv == nil || sv.Kind != VKAttrs {
				return nil, evalErr(pos, "value is not an attribute set")
			}
			b, ok := sv.Attrs.Binds[sym]
			if !ok {
				return nil, evalErr(pos, "attribute %q missing", attr)
			}
			return e.attrBindingValue(b)
		},
	}
	return Deferred(t)
}

func (ev *Evaluator) derivationOutputOverlayLazy(strict *Value, outputName string, pos parser.Pos) *Value {
	overlay := Attrs(false)
	overlay.Attrs.Binds[ev.Intern.Intern("type")] = &AttrBinding{Value: Str("derivation", pos)}
	overlay.Attrs.Binds[ev.Intern.Intern("outputName")] = &AttrBinding{Value: Str(outputName, pos)}
	overlay.Attrs.Binds[ev.Intern.Intern("drvPath")] = &AttrBinding{Value: ev.lazySelectStrictAttr(strict, "drvPath", pos)}
	if outputName == "out" {
		t := &Thunk{
			eval: ev,
			pos:  pos,
			lazy: func(e *Evaluator) (*Value, error) {
				sv, err := e.forceValue(strict)
				if err != nil {
					return nil, err
				}
				if sv == nil || sv.Kind != VKAttrs {
					return nil, evalErr(pos, "value is not an attribute set")
				}
				for _, name := range []string{"outPath", "out"} {
					if b, ok := sv.Attrs.Binds[e.Intern.Intern(name)]; ok {
						return e.attrBindingValue(b)
					}
				}
				return nil, evalErr(pos, "attribute %q missing", "outPath")
			},
		}
		overlay.Attrs.Binds[ev.Intern.Intern("outPath")] = &AttrBinding{Value: Deferred(t)}
	} else {
		overlay.Attrs.Binds[ev.Intern.Intern("outPath")] = &AttrBinding{Value: ev.lazySelectStrictAttr(strict, outputName, pos)}
	}
	return overlay
}

// buildNixDerivation implements Nix primops/derivation.nix (2.24.9).
func (ev *Evaluator) buildNixDerivation(drvAttrs *Value, pos parser.Pos) (*Value, error) {
	strict := ev.lazyStrictDerivation(drvAttrs, pos)
	outputNames, err := ev.derivationOutputNames(drvAttrs, pos)
	if err != nil {
		return nil, err
	}

	common := cloneAttrSetShallow(drvAttrs)
	common.Attrs.Binds[ev.Intern.Intern("drvAttrs")] = &AttrBinding{Value: drvAttrs}

	outputValues := make([]*Value, len(outputNames))
	for i, oname := range outputNames {
		oname := oname
		strictCapture := strict
		t := &Thunk{
			eval: ev,
			pos:  pos,
			lazy: func(e *Evaluator) (*Value, error) {
				ov := e.derivationOutputOverlayLazy(strictCapture, oname, pos)
				return mergeAttrBindMaps(common, ov), nil
			},
		}
		deferred := Deferred(t)
		outputValues[i] = deferred
		common.Attrs.Binds[ev.Intern.Intern(oname)] = &AttrBinding{Value: deferred}
	}
	common.Attrs.Binds[ev.Intern.Intern("all")] = &AttrBinding{Value: List(outputValues...)}

	headOverlay := ev.derivationOutputOverlayLazy(strict, outputNames[0], pos)
	return mergeAttrBindMaps(common, headOverlay), nil
}
