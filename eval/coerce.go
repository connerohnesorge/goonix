// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
)

// CoerceMode selects Nix coerceToString rules for non-string types.
type CoerceMode int

const (
	// CoerceStrict rejects null, bool, int, float, and list (attrsets use __toString / outPath).
	CoerceStrict CoerceMode = iota
	// CoerceString is Nix string/derivation-env/toString coercion: null→"", bool→1/"", numbers, lists joined with spaces.
	CoerceString
)

// CoerceContext configures coerceToString.
type CoerceContext struct {
	Mode CoerceMode
	Pos  parser.Pos
	// CopyPaths copies non-store filesystem paths into the store (derivation env / realised strings).
	CopyPaths bool
}

func copyStringContext(ctx []StringContextElem) []StringContextElem {
	if len(ctx) == 0 {
		return nil
	}
	out := make([]StringContextElem, len(ctx))
	copy(out, ctx)
	return out
}

func mergeStringContexts(a, b []StringContextElem) []StringContextElem {
	if len(a) == 0 {
		return copyStringContext(b)
	}
	if len(b) == 0 {
		return copyStringContext(a)
	}
	out := make([]StringContextElem, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// coerceToString implements Nix EvalState::coerceToString for string contexts.
func (ev *Evaluator) coerceToString(v *Value, cc CoerceContext) (*Value, error) {
	pos := cc.Pos
	if v != nil && pos.File == "" && v.Pos.File != "" {
		pos = v.Pos
	}
	fv, err := ev.mustWHNF(v, pos)
	if err != nil {
		return nil, err
	}
	if fv.Kind == VKBuiltin && fv.Builtin != nil && fv.Builtin.Nullary {
		fv, err = fv.Builtin.Fn(ev, pos, nil)
		if err != nil {
			return nil, err
		}
		fv, err = ev.mustWHNF(fv, pos)
		if err != nil {
			return nil, err
		}
	}

	switch fv.Kind {
	case VKString:
		return StrWithContext(fv.String, copyStringContext(fv.Ctx), pos), nil
	case VKPath:
		return ev.coercePathToString(fv, pos, cc.CopyPaths)
	case VKAttrs:
		return ev.coerceAttrSetToString(fv, cc)
	case VKNull, VKBool, VKInt, VKFloat, VKList:
		if cc.Mode != CoerceString {
			return nil, evalErr(pos, "cannot coerce %s to string", valueKindName(fv.Kind))
		}
		return ev.coerceToStringScalarList(fv, cc)
	default:
		return nil, evalErr(pos, "cannot coerce %s to string", valueKindName(fv.Kind))
	}
}

func (ev *Evaluator) coerceToStringScalarList(fv *Value, cc CoerceContext) (*Value, error) {
	pos := cc.Pos
	switch fv.Kind {
	case VKNull:
		return Str("", pos), nil
	case VKBool:
		if fv.Bool {
			return Str("1", pos), nil
		}
		return Str("", pos), nil
	case VKInt:
		return Str(strconv.FormatInt(fv.Int, 10), pos), nil
	case VKFloat:
		return Str(strconv.FormatFloat(fv.Float, 'g', -1, 64), pos), nil
	case VKList:
		var parts []string
		var ctx []StringContextElem
		for _, el := range fv.List {
			sv, err := ev.coerceToString(el, cc)
			if err != nil {
				return nil, err
			}
			if sv.String != "" {
				parts = append(parts, sv.String)
			}
			ctx = mergeStringContexts(ctx, sv.Ctx)
		}
		return StrWithContext(strings.Join(parts, " "), ctx, pos), nil
	default:
		return nil, evalErr(pos, "cannot coerce %s to string", valueKindName(fv.Kind))
	}
}

func (ev *Evaluator) coerceAttrSetToString(fv *Value, cc CoerceContext) (*Value, error) {
	if fv == nil || fv.Kind != VKAttrs {
		return nil, evalErr(cc.Pos, "cannot coerce %s to string", valueKindName(VKAttrs))
	}
	intern := ev.Intern
	if b := fv.Attrs.Binds[intern.Intern("__toString")]; b != nil {
		tv, err := ev.attrsBindingForce(b)
		if err != nil {
			return nil, err
		}
		result, err := ev.applyCallable(tv, fv, cc.Pos)
		if err != nil {
			return nil, err
		}
		return ev.coerceToString(result, cc)
	}
	if b := fv.Attrs.Binds[intern.Intern("outPath")]; b != nil {
		tv, err := ev.attrsBindingForce(b)
		if err != nil {
			return nil, err
		}
		return ev.coerceToString(tv, cc)
	}
	return nil, evalErr(cc.Pos, "cannot coerce %s to a string", valueKindName(VKAttrs))
}

func (ev *Evaluator) coercePathToString(fv *Value, pos parser.Pos, copyPaths bool) (*Value, error) {
	p := fv.Path
	storeDir := ev.Effects.StoreDir()
	if storeDir == "" {
		storeDir = store.DefaultStoreDir
	}
	ctx := pathStringContext(p, storeDir)
	if !copyPaths || strings.HasPrefix(p, storeDir+"/") || p == storeDir {
		return StrWithContext(p, ctx, pos), nil
	}
	cfg := store.Config{StoreDir: storeDir}
	name := filepath.Base(p)
	if name == "" || name == "." || name == "/" {
		name = "source"
	}
	storePath, err := cfg.StorePathForFilesystemPath(p, name)
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	err = ev.Effects.RealiseStorePath(storePath, func() error {
		return store.CopyPathToStore(p, storePath)
	})
	if err != nil {
		return nil, err
	}
	ctx = []StringContextElem{{StorePath: storePath}}
	return StrWithContext(storePath, ctx, pos), nil
}

func pathStringContext(p, storeDir string) []StringContextElem {
	if storeDir == "" {
		storeDir = store.DefaultStoreDir
	}
	if strings.HasPrefix(p, storeDir+"/") || p == storeDir {
		return []StringContextElem{{StorePath: p}}
	}
	return nil
}

func (ev *Evaluator) concatCoercedStrings(a, b *Value, pos parser.Pos) (*Value, error) {
	cc := CoerceContext{Mode: CoerceString, Pos: pos}
	fa, err := ev.forceValue(a)
	if err != nil {
		return nil, err
	}
	fb, err := ev.forceValue(b)
	if err != nil {
		return nil, err
	}
	if fa.Kind == VKPath || fb.Kind == VKPath {
		if fa.Kind == VKPath && fb.Kind == VKPath {
			return Path(fa.Path + fb.Path), nil
		}
		if fa.Kind == VKPath {
			bs, err := ev.coerceToString(fb, cc)
			if err != nil {
				return nil, err
			}
			return Path(fa.Path + bs.String), nil
		}
		as, err := ev.coerceToString(fa, cc)
		if err != nil {
			return nil, err
		}
		return Path(as.String + fb.Path), nil
	}
	as, err := ev.coerceToString(fa, cc)
	if err != nil {
		return nil, err
	}
	bs, err := ev.coerceToString(fb, cc)
	if err != nil {
		return nil, err
	}
	return StrWithContext(as.String+bs.String, mergeStringContexts(as.Ctx, bs.Ctx), pos), nil
}
