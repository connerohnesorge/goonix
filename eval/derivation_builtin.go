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

func (ev *Evaluator) derivationOutputOverlay(strict *Value, outputName string, pos parser.Pos) *Value {
	if strict == nil || strict.Kind != VKAttrs {
		return Attrs(false)
	}
	overlay := Attrs(false)
	set := func(name string, v *Value) {
		overlay.Attrs.Binds[ev.Intern.Intern(name)] = &AttrBinding{Value: v}
	}
	set("type", Str("derivation", pos))
	set("outputName", Str(outputName, pos))
	drvSym := ev.Intern.Intern("drvPath")
	if b, ok := strict.Attrs.Binds[drvSym]; ok {
		overlay.Attrs.Binds[drvSym] = b
	}
	outSym := ev.Intern.Intern(outputName)
	if b, ok := strict.Attrs.Binds[outSym]; ok {
		// Nix: outPath = getAttr outputName strict (path lives under output name on strict)
		overlay.Attrs.Binds[ev.Intern.Intern("outPath")] = b
	} else if outputName == "out" {
		if b, ok := strict.Attrs.Binds[ev.Intern.Intern("outPath")]; ok {
			overlay.Attrs.Binds[ev.Intern.Intern("outPath")] = b
		}
	}
	return overlay
}

// buildNixDerivation implements Nix primops/derivation.nix (2.24.9).
func (ev *Evaluator) buildNixDerivation(drvAttrs *Value, pos parser.Pos) (*Value, error) {
	strict, err := builtinDerivationStrict(ev, pos, []*Value{drvAttrs})
	if err != nil {
		return nil, err
	}
	outputNames, err := ev.derivationOutputNames(drvAttrs, pos)
	if err != nil {
		return nil, err
	}

	common := cloneAttrSetShallow(drvAttrs)
	common.Attrs.Binds[ev.Intern.Intern("drvAttrs")] = &AttrBinding{Value: drvAttrs}

	outputValues := make([]*Value, len(outputNames))
	for i, oname := range outputNames {
		oname := oname
		t := &Thunk{
			eval: ev,
			pos:  pos,
			lazy: func(e *Evaluator) (*Value, error) {
				ov := e.derivationOutputOverlay(strict, oname, pos)
				return mergeAttrBindMaps(common, ov), nil
			},
		}
		deferred := Deferred(t)
		outputValues[i] = deferred
		common.Attrs.Binds[ev.Intern.Intern(oname)] = &AttrBinding{Value: deferred}
	}
	common.Attrs.Binds[ev.Intern.Intern("all")] = &AttrBinding{Value: List(outputValues...)}

	headOverlay := ev.derivationOutputOverlay(strict, outputNames[0], pos)
	return mergeAttrBindMaps(common, headOverlay), nil
}
