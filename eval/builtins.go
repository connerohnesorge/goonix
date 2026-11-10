// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package eval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	stdsort "sort"
	"strings"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/internal/nixversion"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

// Builtin is a primitive function registered on the global builtin attrset.
type Builtin struct {
	Name    string
	Nullary bool // auto-apply when selected as an attribute (e.g. nixVersion)
	Fn      func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error)
}

func (ev *Evaluator) registerBuiltins() {
	register := func(name string, fn func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error)) {
		ev.Builtins[name] = &Builtin{Name: name, Fn: fn}
	}
	registerNullary := func(name string, fn func(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error)) {
		ev.Builtins[name] = &Builtin{Name: name, Nullary: true, Fn: fn}
	}

	register("import", builtinImport)
	register("toString", builtinToString)
	register("map", builtinMap)
	register("concatMap", builtinConcatMap)
	register("filter", builtinFilter)
	register("sort", builtinSort)
	register("foldl'", builtinFoldlPrime)
	register("attrNames", builtinAttrNames)
	register("catAttrs", builtinCatAttrs)
	register("getAttr", builtinGetAttr)
	register("hasAttr", builtinHasAttr)
	register("removeAttrs", builtinRemoveAttrs)
	register("intersectAttrs", builtinIntersectAttrs)
	register("listToAttrs", builtinListToAttrs)
	register("seq", builtinSeq)
	register("deepSeq", builtinDeepSeq)
	register("tryEval", builtinTryEval)
	register("throw", builtinThrow)
	register("abort", builtinAbort)
	register("trace", builtinTrace)
	register("getEnv", builtinGetEnv)
	register("pathExists", builtinPathExists)
	register("readFile", builtinReadFile)
	register("readDir", builtinReadDir)
	register("hashString", builtinHashString)
	register("match", builtinMatch)
	register("split", builtinSplit)
	register("replaceStrings", builtinReplaceStrings)
	register("fromJSON", builtinFromJSON)
	register("toJSON", builtinToJSON)
	register("storePath", builtinStorePath)
	register("placeholder", builtinPlaceholder)
	register("derivationStrict", builtinDerivationStrict)
	register("derivation", builtinDerivation)
	register("getContext", builtinGetContext)
	register("appendContext", builtinAppendContext)
	register("toFile", builtinToFile)
	register("path", builtinPath)
	register("filterSource", builtinFilterSource)
	registerNullary("nixPath", builtinNixPath)
	register("findFile", builtinFindFile)
	registerNullary("storeDir", builtinStoreDir)
	registerNullary("nixVersion", builtinNixVersion)
	register("compareVersions", builtinCompareVersions)
	register("splitVersion", builtinSplitVersion)
	register("lessThan", builtinLessThan)
	register("add", builtinAdd)
	register("sub", builtinSub)
	registerNullary("currentSystem", builtinCurrentSystem)
	register("fetchurl", builtinFetchurl)
	register("length", builtinLength)
	register("isList", builtinIsList)
	register("isInt", builtinIsInt)
	register("isFloat", builtinIsFloat)
	register("isString", builtinIsString)
	register("isAttrs", builtinIsAttrs)
	register("isFunction", builtinIsFunction)
	register("isPath", builtinIsPath)
	register("typeOf", builtinTypeOf)
	register("substring", builtinSubstring)
	register("stringLength", builtinStringLength)
	register("concatStringsSep", builtinConcatStringsSep)
	register("parseDrvName", builtinParseDrvName)
	register("unsafeDiscardStringContext", builtinUnsafeDiscardStringContext)
	register("head", builtinHead)
	register("tail", builtinTail)
	register("elemAt", builtinElemAt)
	register("concatLists", builtinConcatLists)
	register("elem", builtinElem)
	register("all", builtinAll)
	register("any", builtinAny)
	register("genList", builtinGenList)
	register("mapAttrs", builtinMapAttrs)
	register("attrValues", builtinAttrValues)
	register("zipAttrsWith", builtinZipAttrsWith)
	register("genericClosure", builtinGenericClosure)
	register("isBool", builtinIsBool)
	register("bitAnd", builtinBitAnd)
	register("bitOr", builtinBitOr)
	register("bitXor", builtinBitXor)
	register("baseNameOf", builtinBaseNameOf)
	register("dirOf", builtinDirOf)
	register("mul", builtinMul)
	register("div", builtinDiv)
	register("unsafeGetAttrPos", builtinUnsafeGetAttrPos)
	register("functionArgs", builtinFunctionArgs)
	register("addErrorContext", builtinAddErrorContext)
	ev.registerExtendedBuiltins()
}

// BuiltinAttrSet returns an attribute set of all registered builtins.
func (ev *Evaluator) BuiltinAttrSet() *Value {
	av := Attrs(false)
	for name, b := range ev.Builtins {
		sym := ev.Intern.Intern(name)
		av.Attrs.Binds[sym] = &AttrBinding{Value: &Value{Kind: VKBuiltin, Builtin: b}}
	}
	return av
}

func arity(pos parser.Pos, args []*Value, n int) error {
	if len(args) != n {
		return evalErr(pos, "expected %d arguments, got %d", n, len(args))
	}
	return nil
}

func builtinImport(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) < 1 || len(args) > 2 {
		return nil, evalErr(pos, "expected 1 or 2 arguments, got %d", len(args))
	}
	path, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	if (strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")) && ev.curFile != "" {
		path = filepath.Join(filepath.Dir(ev.curFile), path)
	}
	path = resolveImportPath(path)
	moduleVal, err := ev.importModule(path)
	if err != nil {
		return nil, err
	}
	if len(args) == 1 {
		return moduleVal, nil
	}
	return ev.applyCall(moduleVal, args[1], pos)
}

func (ev *Evaluator) importModule(path string) (*Value, error) {
	if cached, ok := ev.importCache[path]; ok {
		return cached, nil
	}
	expr, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	prev := ev.curFile
	ev.curFile = path
	defer func() { ev.curFile = prev }()
	env := NewEnv()
	env.bind(ev.Intern.Intern("builtins"), ev.BuiltinAttrSet())
	ev.bindCorepkgsEnv(env)
	moduleVal, err := ev.evalExpr(expr, env)
	if err != nil {
		return nil, err
	}
	ev.importCache[path] = moduleVal
	return moduleVal, nil
}

func (ev *Evaluator) bindCorepkgsEnv(env *Env) {
	for _, name := range []string{"derivation", "baseNameOf", "toString", "dirOf"} {
		if b, ok := ev.Builtins[name]; ok {
			env.bind(ev.Intern.Intern(name), &Value{Kind: VKBuiltin, Builtin: b})
		}
	}
}

func resolveImportPath(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return path
	}
	if st.IsDir() {
		return filepath.Join(path, "default.nix")
	}
	return path
}

func builtinToString(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	sv, err := ev.coerceToString(args[0], CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: false})
	if err != nil {
		return nil, err
	}
	return StrWithContext(sv.String, copyStringContext(sv.Ctx), pos), nil
}

func builtinMap(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	out := make([]*Value, len(list))
	for i, el := range list {
		out[i] = ev.deferredApplyCall(f, el, pos)
	}
	return List(out...), nil
}

func builtinConcatMap(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	var out []*Value
	for _, el := range list {
		r := ev.deferredApplyCall(f, el, pos)
		sub, err := forceList(ev, pos, r)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return List(out...), nil
}

func (ev *Evaluator) applyCall(f, arg *Value, pos parser.Pos) (*Value, error) {
	return ev.applyCallable(f, arg, pos)
}

func builtinFilter(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	pred, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	lv, err := ev.mustWHNF(args[1], pos)
	if err != nil {
		return nil, err
	}
	var elems []*Value
	if lv.Kind == VKNull {
		elems = nil
	} else if lv.Kind != VKList {
		return nil, evalErr(pos, "filter expects a list")
	} else {
		elems = lv.List
	}
	var out []*Value
	for _, el := range elems {
		r, err := ev.applyCall(pred, el, pos)
		if err != nil {
			return nil, err
		}
		b, err := ev.forceBool(ev, r, nil)
		if err != nil {
			return nil, err
		}
		if b {
			out = append(out, el)
		}
	}
	return List(out...), nil
}

func builtinSort(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	comp, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	items := append([]*Value{}, list...)
	stdsort.SliceStable(items, func(i, j int) bool {
		r, err := ev.applyCall2(comp, items[i], items[j], pos)
		if err != nil {
			return false
		}
		b, err := ev.forceBool(ev, r, nil)
		if err != nil {
			return false
		}
		return b
	})
	return List(items...), nil
}

func builtinFoldlPrime(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 3); err != nil {
		return nil, err
	}
	op, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	acc := args[1]
	list, err := forceList(ev, pos, args[2])
	if err != nil {
		return nil, err
	}
	for _, el := range list {
		r, err := ev.applyCall2(op, acc, el, pos)
		if err != nil {
			return nil, err
		}
		acc, err = ev.forceValue(r)
		if err != nil {
			return nil, err
		}
	}
	return acc, nil
}

func (ev *Evaluator) applyCall2(f, a, b *Value, pos parser.Pos) (*Value, error) {
	r, err := ev.applyCallable(f, a, pos)
	if err != nil {
		return nil, err
	}
	return ev.applyCallable(r, b, pos)
}

func builtinAttrNames(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[0], pos)
	if err != nil {
		return nil, err
	}
	var names []string
	for sym := range set.Attrs.Binds {
		names = append(names, ev.Intern.Lookup(sym))
	}
	// sort for stability
	stringsSort(names)
	elems := make([]*Value, len(names))
	for i, n := range names {
		elems[i] = Str(n, pos)
	}
	return List(elems...), nil
}

func builtinCatAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	sym := ev.Intern.Intern(name)
	var out []*Value
	for _, el := range list {
		set, err := ev.forceAttrs(el, pos)
		if err != nil {
			continue
		}
		if b := set.Attrs.Binds[sym]; b != nil {
			if b.Thunk != nil {
				out = append(out, Deferred(b.Thunk))
			} else {
				out = append(out, b.Value)
			}
		}
	}
	return List(out...), nil
}

func stringsSort(ss []string) {
	for i := 0; i < len(ss); i++ {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] < ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}

func builtinGetAttr(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[1], pos)
	if err != nil {
		return nil, err
	}
	sym := ev.Intern.Intern(name)
	b, ok := set.Attrs.Binds[sym]
	if !ok {
		return nil, evalErr(pos, "attribute %q missing", name)
	}
	if b.Thunk != nil {
		return Deferred(b.Thunk), nil
	}
	return b.Value, nil
}

func builtinHasAttr(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[1], pos)
	if err != nil {
		return nil, err
	}
	sym := ev.Intern.Intern(name)
	_, ok := set.Attrs.Binds[sym]
	return Bool(ok), nil
}

func builtinRemoveAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[0], pos)
	if err != nil {
		return nil, err
	}
	names, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	out := Attrs(false)
removeLoop:
	for sym, bind := range set.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		for _, el := range names {
			s, err := ev.forceString(el)
			if err != nil {
				return nil, err
			}
			if s == name {
				continue removeLoop
			}
		}
		out.Attrs.Binds[sym] = bind
	}
	return out, nil
}

func builtinIntersectAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	lbinds, err := ev.attrsBindingMap(args[0], pos)
	if err != nil {
		return nil, err
	}
	rbinds, err := ev.attrsBindingMap(args[1], pos)
	if err != nil {
		return nil, err
	}
	out := Attrs(false)
	for sym := range lbinds {
		if bind, ok := rbinds[sym]; ok {
			out.Attrs.Binds[sym] = bind
		}
	}
	return out, nil
}

// attrsBindingMap returns attr bindings without forcing every attribute value.
func (ev *Evaluator) attrsBindingMap(v *Value, pos parser.Pos) (map[data.Symbol]*AttrBinding, error) {
	av, err := ev.forceAttrs(v, pos)
	if err != nil {
		return nil, err
	}
	if av.Attrs == nil {
		return make(map[data.Symbol]*AttrBinding), nil
	}
	return av.Attrs.Binds, nil
}

func builtinListToAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	out := Attrs(false)
	for _, el := range list {
		item, err := ev.forceAttrs(el, pos)
		if err != nil {
			return nil, err
		}
		nameSym := ev.Intern.Intern("name")
		valSym := ev.Intern.Intern("value")
		nb, ok := item.Attrs.Binds[nameSym]
		if !ok {
			return nil, evalErr(pos, "missing name attribute")
		}
		vb, ok := item.Attrs.Binds[valSym]
		if !ok {
			return nil, evalErr(pos, "missing value attribute")
		}
		var ns string
		if nb.Thunk != nil {
			nv, err := ev.forceThunk(nb.Thunk)
			if err != nil {
				return nil, err
			}
			ns, err = ev.forceString(nv)
		} else {
			ns, err = ev.forceString(nb.Value)
		}
		if err != nil {
			return nil, err
		}
		// Nix reports the position of the `value` attribute.
		if vb.Thunk != nil {
			out.Attrs.Binds[ev.Intern.Intern(ns)] = &AttrBinding{Thunk: vb.Thunk, Pos: vb.Pos}
		} else {
			out.Attrs.Binds[ev.Intern.Intern(ns)] = &AttrBinding{Value: vb.Value, Pos: vb.Pos}
		}
	}
	return out, nil
}

func builtinSeq(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	if _, err := ev.forceValue(args[0]); err != nil {
		return nil, err
	}
	return args[1], nil
}

func builtinDeepSeq(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	if err := ev.deepForce(args[0]); err != nil {
		return nil, err
	}
	return args[1], nil
}

func (ev *Evaluator) deepForce(v *Value) error {
	fv, err := ev.forceValue(v)
	if err != nil {
		return err
	}
	switch fv.Kind {
	case VKList:
		for _, el := range fv.List {
			if err := ev.deepForce(el); err != nil {
				return err
			}
		}
	case VKAttrs:
		for _, bind := range fv.Attrs.Binds {
			if bind.Thunk != nil {
				val, err := ev.forceThunk(bind.Thunk)
				if err != nil {
					return err
				}
				if err := ev.deepForce(val); err != nil {
					return err
				}
				continue
			}
			if err := ev.deepForce(bind.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ev *Evaluator) deepForceValue(v *Value) (*Value, error) {
	if err := ev.deepForce(v); err != nil {
		return nil, err
	}
	return ev.forceValue(v)
}

func builtinTryEval(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	v, err := ev.forceValue(args[0])
	out := Attrs(false)
	successSym := ev.Intern.Intern("success")
	valueSym := ev.Intern.Intern("value")
	if err != nil {
		out.Attrs.Binds[successSym] = &AttrBinding{Value: Bool(false)}
		out.Attrs.Binds[valueSym] = &AttrBinding{Value: Str(err.Error(), pos)}
		return out, nil
	}
	out.Attrs.Binds[successSym] = &AttrBinding{Value: Bool(true)}
	out.Attrs.Binds[valueSym] = &AttrBinding{Value: v}
	return out, nil
}

func builtinThrow(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	msg, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return nil, evalErr(pos, "%s", msg)
}

func builtinAbort(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return builtinThrow(ev, pos, args)
}

func builtinTrace(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	msg, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(os.Stderr, "trace:", msg)
	return args[1], nil
}

func builtinGetEnv(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return Str(os.Getenv(name), pos), nil
}

func builtinPathExists(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	_, err = ev.Effects.Stat(p)
	return Bool(err == nil), nil
}

func builtinReadFile(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.Effects.ReadFile(p)
	if err != nil {
		return nil, err
	}
	return Str(string(b), pos), nil
}

func builtinReadDir(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	// nix-instantiate does not register filterSource/path results, so
	// realising their store context fails before the directory is read.
	if fv.Kind == VKString {
		for _, c := range fv.Ctx {
			sp := c.StorePath
			if sp == "" {
				sp = c.DrvPath
			}
			if sp != "" {
				return nil, evalErr(pos, "path '%s' is not valid", sp)
			}
		}
	}
	p, err := ev.forceString(fv)
	if err != nil {
		return nil, err
	}
	entries, err := ev.Effects.ReadDir(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, evalErr(pos, "reading directory %s: No such file or directory", p)
		}
		return nil, err
	}
	out := Attrs(false)
	for _, e := range entries {
		typ := readDirEntryType(e)
		sym := ev.Intern.Intern(e.Name())
		out.Attrs.Binds[sym] = &AttrBinding{Value: Str(typ, pos)}
	}
	return out, nil
}

func readDirEntryType(e fs.DirEntry) string {
	switch {
	case e.IsDir():
		return "directory"
	case e.Type()&fs.ModeSymlink != 0:
		return "symlink"
	case e.Type().IsRegular():
		return "regular"
	default:
		return "unknown"
	}
}

func builtinHashString(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	algo, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	h, err := store.HashStringDigest(algo, s)
	if err != nil {
		return nil, evalErr(pos, "%v", err)
	}
	return Str(h, pos), nil
}

func builtinMatch(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	re, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	groups, ok, err := nixRegexMatch(re, s)
	if err != nil {
		return nil, evalErr(pos, "%v", err)
	}
	if !ok {
		return Null(), nil
	}
	return matchGroupsToValues(ev, groups, pos)
}

func builtinSplit(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	sep, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	parts, err := nixRegexSplit(sep, s)
	if err != nil {
		return nil, evalErr(pos, "invalid regex: %v", err)
	}
	return nixSplitToValues(ev, parts, pos)
}

func builtinReplaceStrings(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 3); err != nil {
		return nil, err
	}
	fromList, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	toList, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	if len(fromList) != len(toList) {
		return nil, evalErr(pos, "replaceStrings: from and to must be the same length")
	}
	sv, err := ev.coerceToString(args[2], CoerceContext{Mode: CoerceString, Pos: pos})
	if err != nil {
		return nil, err
	}
	out := sv.String
	for i := range fromList {
		from, err := ev.forceString(fromList[i])
		if err != nil {
			return nil, err
		}
		to, err := ev.forceString(toList[i])
		if err != nil {
			return nil, err
		}
		out = strings.ReplaceAll(out, from, to)
	}
	return StrWithContext(out, copyStringContext(sv.Ctx), pos), nil
}

func builtinFromJSON(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	var raw any
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, evalErr(pos, "invalid JSON: %v", err)
	}
	return jsonToValue(ev, raw, pos), nil
}

func jsonToValue(ev *Evaluator, raw any, pos parser.Pos) *Value {
	switch t := raw.(type) {
	case nil:
		return Null()
	case bool:
		return Bool(t)
	case float64:
		if t == float64(int64(t)) {
			return Int(int64(t))
		}
		return Float(t)
	case string:
		return Str(t, pos)
	case []any:
		elems := make([]*Value, len(t))
		for i, el := range t {
			elems[i] = jsonToValue(ev, el, pos)
		}
		return List(elems...)
	case map[string]any:
		av := Attrs(false)
		for k, v := range t {
			av.Attrs.Binds[ev.Intern.Intern(k)] = &AttrBinding{Value: jsonToValue(ev, v, pos)}
		}
		return av
	default:
		return Null()
	}
}

func builtinToJSON(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	v, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	raw, ctx, err := valueToJSON(ev, v)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return StrWithContext(string(b), ctx, pos), nil
}

func valueToJSON(ev *Evaluator, v *Value) (any, []StringContextElem, error) {
	fv, err := ev.forceValue(v)
	if err != nil {
		return nil, nil, err
	}
	switch fv.Kind {
	case VKNull:
		return nil, nil, nil
	case VKBool:
		return fv.Bool, nil, nil
	case VKInt:
		return fv.Int, nil, nil
	case VKFloat:
		if math.IsNaN(fv.Float) || math.IsInf(fv.Float, 0) {
			return nil, nil, nil
		}
		return fv.Float, nil, nil
	case VKString:
		return fv.String, copyStringContext(fv.Ctx), nil
	case VKPath:
		return ev.jsonCopiedPath(fv.Path)
	case VKList:
		out := make([]any, len(fv.List))
		var ctx []StringContextElem
		for i, el := range fv.List {
			x, c, err := valueToJSON(ev, el)
			if err != nil {
				return nil, nil, err
			}
			out[i] = x
			ctx = mergeStringContexts(ctx, c)
		}
		return out, ctx, nil
	case VKAttrs:
		if s, c, ok, err := ev.jsonAttrsCoerce(fv); err != nil || ok {
			return s, c, err
		}
		m := make(map[string]any)
		var ctx []StringContextElem
		for sym, bind := range fv.Attrs.Binds {
			name := ev.Intern.Lookup(sym)
			var el *Value
			if bind.Thunk != nil {
				el, err = ev.forceValue(Deferred(bind.Thunk))
			} else {
				el = bind.Value
			}
			if err != nil {
				return nil, nil, err
			}
			x, c, err := valueToJSON(ev, el)
			if err != nil {
				return nil, nil, err
			}
			m[name] = x
			ctx = mergeStringContexts(ctx, c)
		}
		return m, ctx, nil
	case VKLambda, VKBuiltin, VKPartialBuiltin:
		return nil, nil, fmt.Errorf("cannot convert a function to JSON")
	default:
		return nil, nil, fmt.Errorf("cannot convert a %s to JSON", valueKindName(fv.Kind))
	}
}

// jsonAttrsCoerce applies Nix printValueAsJSON's attrset rules.
// __toString is called and the result is coerced to a string. Otherwise an
// outPath attribute is printed on its own, which is why a derivation does
// not walk the `out` attribute back into itself.
func (ev *Evaluator) jsonAttrsCoerce(fv *Value) (any, []StringContextElem, bool, error) {
	if b := fv.Attrs.Binds[ev.Intern.Intern("__toString")]; b != nil {
		tv, err := ev.attrsBindingForce(b)
		if err != nil {
			return nil, nil, true, err
		}
		result, err := ev.applyCallable(tv, fv, parser.Pos{})
		if err != nil {
			return nil, nil, true, err
		}
		result, err = ev.forceValue(result)
		if err != nil {
			return nil, nil, true, err
		}
		if result.Kind == VKPath {
			s, ctx, err := ev.jsonCopiedPath(result.Path)
			return s, ctx, true, err
		}
		sv, err := ev.coerceToString(result, CoerceContext{Mode: CoerceString, Pos: parser.Pos{}, CopyPaths: true})
		if err != nil {
			return nil, nil, true, err
		}
		return sv.String, copyStringContext(sv.Ctx), true, nil
	}
	if b := fv.Attrs.Binds[ev.Intern.Intern("outPath")]; b != nil {
		tv, err := ev.attrsBindingForce(b)
		if err != nil {
			return nil, nil, true, err
		}
		s, ctx, err := valueToJSON(ev, tv)
		return s, ctx, true, err
	}
	return nil, nil, false, nil
}

func (ev *Evaluator) jsonCopiedPath(p string) (string, []StringContextElem, error) {
	storeDir := ev.Effects.StoreDir()
	if storeDir == "" {
		storeDir = store.DefaultStoreDir
	}
	if p == storeDir || strings.HasPrefix(p, storeDir+"/") {
		return p, []StringContextElem{{StorePath: p}}, nil
	}
	name := filepath.Base(p)
	if name == "" || name == "." || name == "/" {
		name = "source"
	}
	v, err := ev.materializeTreeSource(p, name)
	if err != nil {
		return "", nil, err
	}
	return v.String, copyStringContext(v.Ctx), nil
}

func builtinStorePath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(p, ev.Effects.StoreDir()+"/") {
		return nil, evalErr(pos, "path '%s' is not in the Nix store", p)
	}
	return StrWithContext(p, []StringContextElem{{StorePath: p}}, pos), nil
}

func builtinPlaceholder(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	out, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return Str(store.OutputPlaceholder(out), pos), nil
}

func builtinDerivationStrict(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	attrs, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if attrs.Kind != VKAttrs {
		return nil, evalErr(pos, "derivationStrict expects attribute set")
	}
	drv, err := ev.attrsToDerivation(attrs)
	if err != nil {
		return nil, err
	}
	names, err := ev.derivationOutputNames(attrs, pos)
	if err != nil {
		return nil, err
	}
	return ev.strictDerivationResultAttrs(drv, names, pos), nil
}

func (ev *Evaluator) strictDerivationResultAttrs(drv *DerivationValue, outputNames []string, pos parser.Pos) *Value {
	av := Attrs(false)
	set := func(name string, v *Value) {
		av.Attrs.Binds[ev.Intern.Intern(name)] = &AttrBinding{Value: v}
	}
	set("drvPath", Str(drv.DrvPath, parser.Pos{}))
	for _, outName := range outputNames {
		p := drv.OutPaths[outName]
		ctx := []StringContextElem{{DrvPath: drv.DrvPath, Output: outName}}
		set(outName, StrWithContext(p, ctx, pos))
	}
	return av
}

func builtinDerivation(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	drvAttrs, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if drvAttrs.Kind != VKAttrs {
		return nil, evalErr(pos, "derivation expects attribute set")
	}
	return ev.buildNixDerivation(drvAttrs, pos)
}

func (ev *Evaluator) attrsToDerivation(attrs *Value) (*DerivationValue, error) {
	if ev.forceTrace {
		ev.forceFrames = append(ev.forceFrames, "derivationStrict")
		defer func() { ev.forceFrames = ev.forceFrames[:len(ev.forceFrames)-1] }()
	}
	if attrs != nil && attrs.Kind == VKAttrs && attrs.Attrs != nil {
		if ev.drvStrictByAttrs == nil {
			ev.drvStrictByAttrs = make(map[*AttrSet]*DerivationValue)
		}
		if cached, ok := ev.drvStrictByAttrs[attrs.Attrs]; ok && cached != nil {
			if ev.verifyStrictMemo {
				if err := ev.verifyStrictMemoMatch(attrs, cached); err != nil {
					return nil, err
				}
			}
			return cached, nil
		}
	}
	return ev.computeAttrsToDerivation(attrs)
}

func (ev *Evaluator) computeAttrsToDerivation(attrs *Value) (*DerivationValue, error) {
	pos := parser.Pos{}
	getS := func(name string) (string, error) {
		sym := ev.Intern.Intern(name)
		b, ok := attrs.Attrs.Binds[sym]
		if !ok {
			return "", fmt.Errorf("missing attribute %q", name)
		}
		v, err := ev.attrsBindingForce(b)
		if err != nil {
			return "", err
		}
		col := newDerivationInputCollector()
		s, _, err := ev.coerceDerivationEnvString(v, pos, col)
		return s, err
	}
	name, err := getS("name")
	if err != nil {
		return nil, err
	}
	if err := store.ValidateDerivationName(name); err != nil {
		return nil, err
	}
	drv := &DerivationValue{Name: name}
	if err := ev.fillDerivationFromAttrs(attrs, drv, pos); err != nil {
		return nil, err
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	drvPath, outPaths, err := computeDerivationPaths(ev, cfg, drv)
	if err != nil {
		return nil, err
	}
	drv.DrvPath = drvPath
	drv.OutPaths = outPaths
	ev.rememberStrictDerivation(drv)
	if attrs != nil && attrs.Kind == VKAttrs && attrs.Attrs != nil && ev.drvStrictByAttrs != nil {
		ev.drvStrictByAttrs[attrs.Attrs] = drv
	}
	return drv, nil
}

func (ev *Evaluator) derivationValueToAttrs(drv *DerivationValue) *Value {
	av := Attrs(false)
	set := func(name string, v *Value) {
		av.Attrs.Binds[ev.Intern.Intern(name)] = &AttrBinding{Value: v}
	}
	set("type", Str("derivation", parser.Pos{}))
	set("name", Str(drv.Name, parser.Pos{}))
	set("system", Str(drv.System, parser.Pos{}))
	set("builder", Str(drv.Builder, parser.Pos{}))
	set("drvPath", Str(drv.DrvPath, parser.Pos{}))
	if drv.OutPaths != nil {
		for outName, p := range drv.OutPaths {
			ctx := []StringContextElem{{DrvPath: drv.DrvPath, Output: outName}}
			set(outName, StrWithContext(p, ctx, parser.Pos{}))
		}
		if p, ok := drv.OutPaths["out"]; ok {
			set("outPath", StrWithContext(p, []StringContextElem{{DrvPath: drv.DrvPath, Output: "out"}}, parser.Pos{}))
		}
	}
	args := make([]*Value, len(drv.Args))
	for i, a := range drv.Args {
		args[i] = Str(a, parser.Pos{})
	}
	set("args", List(args...))
	env := Attrs(false)
	for k, v := range drv.Env {
		env.Attrs.Binds[ev.Intern.Intern(k)] = &AttrBinding{Value: Str(v, parser.Pos{})}
	}
	set("env", env)
	return av
}

func builtinGetContext(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	if fv.Kind == VKPath {
		return nil, evalErr(pos, "expected a string but found a path: %s", fv.Path)
	}
	sv, err := ev.coerceToString(args[0], CoerceContext{Mode: CoerceString, Pos: pos})
	if err != nil {
		return nil, err
	}
	av := Attrs(false)
	for _, elem := range sv.Ctx {
		key := elem.DrvPath
		if key == "" {
			key = elem.StorePath
		}
		if key == "" {
			continue
		}
		entry := Attrs(false)
		if elem.StorePath != "" && elem.DrvPath == "" {
			entry.Attrs.Binds[ev.Intern.Intern("path")] = &AttrBinding{Value: Bool(true)}
		} else {
			out := elem.Output
			if out == "" {
				out = "out"
			}
			entry.Attrs.Binds[ev.Intern.Intern("outputs")] = &AttrBinding{Value: List(Str(out, pos))}
		}
		av.Attrs.Binds[ev.Intern.Intern(key)] = &AttrBinding{Value: entry}
	}
	return av, nil
}

func builtinAppendContext(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	return args[0], nil
}

func builtinToFile(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	contentV, err := ev.coerceToString(args[1], CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: true})
	if err != nil {
		return nil, err
	}
	for _, elem := range contentV.Ctx {
		if elem.DrvPath == "" {
			continue
		}
		return nil, evalErr(pos, "files created by builtins.toFile may not reference derivations, but %s references %s", name, nixContextElemString(elem))
	}
	content := contentV.String
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	storePath, err := cfg.StorePathForText(name, content)
	if err != nil {
		return nil, err
	}
	err = ev.Effects.RealiseStorePath(storePath, func() error {
		return store.WriteTextToStore(ev.physicalStorePath(storePath), content)
	})
	if err != nil {
		return nil, err
	}
	return Path(storePath), nil
}

// nixContextElemString is Nix's NixStringContextElem::to_string for a derivation reference.
// StorePath::to_string is the basename, so a built output is "!out!<hash>-name.drv".
func nixContextElemString(elem StringContextElem) string {
	out := elem.Output
	if out == "" {
		out = "out"
	}
	return "!" + out + "!" + filepath.Base(elem.DrvPath)
}

func builtinPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "expected a set but found a %s", valueKindName(set.Kind))
	}
	p, err := getAttrString(ev, set, "path")
	if err != nil {
		return nil, err
	}
	addedFrom := p
	name, err := getAttrString(ev, set, "name")
	if err != nil || name == "" {
		name = filepath.Base(p)
	}
	// Nix follows a top-level symlink and stores the target under the
	// symlink's name. A dangling link is "path '…' does not exist".
	resolved, err := followTopSymlink(p)
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	p = resolved
	recursive := true
	if rec, ok, err := getAttrBool(ev, set, "recursive"); err != nil {
		return nil, err
	} else if ok {
		recursive = rec
	}
	if !recursive {
		info, serr := os.Lstat(p)
		if serr != nil {
			return nil, serr
		}
		if !info.Mode().IsRegular() {
			// Nix 2.24.9: addToStore on a directory with a flat method.
			return nil, evalErr(pos, "reading from file '%s': Is a directory", p)
		}
		return ev.materializeFlatFile(pos, p, name, optionalAttrString(ev, set, "sha256"), addedFrom)
	}
	if filter, ok, err := attrValue(ev, set, "filter"); err != nil {
		return nil, err
	} else if ok {
		return ev.filterSourceToStore(pos, filter, p, name)
	}
	if sha, err := getAttrString(ev, set, "sha256"); err == nil && sha != "" {
		if err := verifyTreeHash(p, sha); err != nil {
			return nil, evalErr(pos, "store path mismatch in (possibly filtered) path added from '%s'", addedFrom)
		}
	}
	return ev.materializeTreeSource(p, name)
}

func followTopSymlink(p string) (string, error) {
	cur := p
	for i := 0; i < 64; i++ {
		info, err := os.Lstat(cur)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("path '%s' does not exist", cur)
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return cur, nil
		}
		target, err := os.Readlink(cur)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			cur = target
		} else {
			cur = filepath.Join(filepath.Dir(cur), target)
		}
	}
	return "", fmt.Errorf("too many levels of symbolic links")
}

func builtinFilterSource(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	src, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	name := filepath.Base(src)
	return ev.filterSourceToStore(pos, args[0], src, name)
}

func builtinNixPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) != 0 {
		return nil, evalErr(pos, "nixPath expects no arguments")
	}
	entries := ev.NixPath.Entries()
	elems := make([]*Value, len(entries))
	for i, e := range entries {
		elems[i] = Str(e, pos)
	}
	return List(elems...), nil
}

func builtinFindFile(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	p, err := ev.NixPath.FindFile(name)
	if err != nil {
		return nil, err
	}
	return Path(p), nil
}

func builtinStoreDir(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) != 0 {
		return nil, evalErr(pos, "storeDir expects no arguments")
	}
	return Str(ev.Effects.StoreDir(), pos), nil
}

func builtinCompareVersions(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	v1, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	v2, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	return Int(int64(compareVersions(v1, v2))), nil
}

func builtinSplitVersion(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	v, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return List(splitVersion(v)...), nil
}

func builtinNixVersion(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return Str(nixversion.Version, pos), nil
}

func builtinCurrentSystem(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return Str(currentSystemString(), pos), nil
}

func currentSystemString() string {
	switch runtime.GOOS {
	case "linux":
		return "x86_64-linux"
	case "darwin":
		return "aarch64-darwin"
	default:
		return runtime.GOOS + "-" + runtime.GOARCH
	}
}

func builtinFetchurl(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	params, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	var rawURL, name, wantHash string
	switch params.Kind {
	case VKString:
		rawURL = params.String
		name = filepath.Base(rawURL)
	case VKPath:
		rawURL = params.Path
		name = filepath.Base(rawURL)
	case VKAttrs:
		rawURL, err = getAttrString(ev, params, "url")
		if err != nil {
			return nil, err
		}
		name = filepath.Base(rawURL)
		if n, nerr := getAttrString(ev, params, "name"); nerr == nil && n != "" {
			name = n
		}
		wantHash = optionalAttrString(ev, params, "sha256")
	default:
		return nil, evalErr(pos, "expected a set but found a %s", valueKindName(params.Kind))
	}
	data, err := fetchURLBytes(rawURL)
	if err != nil {
		return nil, err
	}
	if want := wantHash; want != "" {
		sum := sha256.Sum256(data)
		got, gerr := nixhash.NewHash(nixhash.SHA256, sum[:])
		if gerr != nil {
			return nil, gerr
		}
		hashText := want
		if !strings.Contains(want, ":") && !strings.HasPrefix(want, "sha256-") && len(want) == 64 {
			hashText = "sha256:" + want
		}
		specified, serr := store.ParseHash(hashText)
		if serr != nil {
			return nil, serr
		}
		if specified.Format(nixhash.Base16, false) != got.Format(nixhash.Base16, false) {
			return nil, evalErr(pos, "hash mismatch in file downloaded from '%s':\n  specified: %s\n  got:       %s", rawURL, specified.Format(nixhash.NixBase32, true), got.Format(nixhash.NixBase32, true))
		}
	}
	sum := sha256.Sum256(data)
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	logical, err := store.FlatSHA256OutputPath(cfg, name, hex.EncodeToString(sum[:]))
	if err != nil {
		return nil, err
	}
	phys := ev.physicalStorePath(logical)
	if err := ev.Effects.RealiseStorePath(logical, func() error {
		return store.WriteTextToStore(phys, string(data))
	}); err != nil {
		return nil, err
	}
	return StrWithContext(logical, []StringContextElem{{StorePath: logical}}, pos), nil
}

func optionalAttrString(ev *Evaluator, set *Value, name string) string {
	s, err := getAttrString(ev, set, name)
	if err != nil {
		return ""
	}
	return s
}

func getAttrBool(ev *Evaluator, set *Value, name string) (bool, bool, error) {
	v, ok, err := attrValue(ev, set, name)
	if err != nil || !ok {
		return false, ok, err
	}
	b, err := ev.forceBool(ev, v, nil)
	return b, true, err
}

func (ev *Evaluator) materializeFlatFile(pos parser.Pos, path, name, sha, addedFrom string) (*Value, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	if sha != "" {
		hashText := sha
		if !strings.Contains(sha, ":") && !strings.HasPrefix(sha, "sha256-") && len(sha) == 64 {
			hashText = "sha256:" + sha
		}
		specified, serr := store.ParseHash(hashText)
		if serr != nil {
			return nil, serr
		}
		got, gerr := nixhash.NewHash(nixhash.SHA256, sum[:])
		if gerr != nil {
			return nil, gerr
		}
		if specified.Format(nixhash.Base16, false) != got.Format(nixhash.Base16, false) {
			from := addedFrom
			if from == "" {
				from = path
			}
			return nil, evalErr(pos, "store path mismatch in (possibly filtered) path added from '%s'", from)
		}
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	logical, err := store.FlatSHA256OutputPath(cfg, name, hex.EncodeToString(sum[:]))
	if err != nil {
		return nil, err
	}
	phys := ev.physicalStorePath(logical)
	if err := ev.Effects.RealiseStorePath(logical, func() error {
		return store.WriteTextToStore(phys, string(data))
	}); err != nil {
		return nil, err
	}
	return StrWithContext(logical, []StringContextElem{{StorePath: logical}}, pos), nil
}

func getAttrString(ev *Evaluator, set *Value, name string) (string, error) {
	sym := ev.Intern.Intern(name)
	b, ok := set.Attrs.Binds[sym]
	if !ok {
		return "", fmt.Errorf("missing attribute %q", name)
	}
	var v *Value
	var err error
	if b.Thunk != nil {
		v, err = ev.forceThunk(b.Thunk)
	} else {
		v = b.Value
	}
	if err != nil {
		return "", err
	}
	return ev.forceString(v)
}

func forceList(ev *Evaluator, pos parser.Pos, v *Value) ([]*Value, error) {
	fv, err := ev.mustWHNF(v, pos)
	if err != nil {
		return nil, err
	}
	if fv.Kind != VKList {
		ev.logTypeMismatchWant(pos, fv, "list")
		return nil, evalErr(pos, "expected a list")
	}
	return fv.List, nil
}

func builtinLength(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	return Int(int64(len(list))), nil
}

func builtinIsList(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKList), nil
}

func builtinHead(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, evalErr(pos, "head called on empty list")
	}
	return list[0], nil
}

func builtinTail(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, evalErr(pos, "tail called on empty list")
	}
	return List(list[1:]...), nil
}

func builtinElemAt(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	idx, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if idx.Kind != VKInt {
		return nil, evalErr(pos, "elemAt index must be integer")
	}
	i := idx.Int
	if i < 0 {
		i += int64(len(list))
	}
	if i < 0 || int(i) >= len(list) {
		return nil, evalErr(pos, "elemAt: index out of bounds")
	}
	return list[i], nil
}

func builtinConcatLists(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	lists, err := forceList(ev, pos, args[0])
	if err != nil {
		return nil, err
	}
	var out []*Value
	for _, el := range lists {
		sub, err := forceList(ev, pos, el)
		if err != nil {
			return nil, err
		}
		out = append(out, sub...)
	}
	return List(out...), nil
}

func builtinElem(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	needle, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	for _, el := range list {
		eq, err := ev.equalValues(needle, el)
		if err != nil {
			return nil, err
		}
		if eq {
			return Bool(true), nil
		}
	}
	return Bool(false), nil
}

func builtinAll(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	pred, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	for _, el := range list {
		r, err := ev.applyCall(pred, el, pos)
		if err != nil {
			return nil, err
		}
		b, err := ev.forceBool(ev, r, nil)
		if err != nil {
			return nil, err
		}
		if !b {
			return Bool(false), nil
		}
	}
	return Bool(true), nil
}

func builtinAny(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	pred, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	for _, el := range list {
		r, err := ev.applyCall(pred, el, pos)
		if err != nil {
			return nil, err
		}
		b, err := ev.forceBool(ev, r, nil)
		if err != nil {
			return nil, err
		}
		if b {
			return Bool(true), nil
		}
	}
	return Bool(false), nil
}

func builtinGenList(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	n, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if n.Kind != VKInt {
		return nil, evalErr(pos, "genList length must be integer")
	}
	out := make([]*Value, n.Int)
	for i := int64(0); i < n.Int; i++ {
		out[i] = ev.deferredApplyCall(f, Int(i), pos)
	}
	return List(out...), nil
}

func builtinMapAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[1], pos)
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "mapAttrs expects attribute set")
	}
	out := Attrs(false)
	for sym, bind := range set.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		inBind := bind
		nameVal := Str(name, pos)
		t := &Thunk{
			eval: ev,
			pos:  pos,
			lazy: func(ev *Evaluator) (*Value, error) {
				var val *Value
				var err error
				if inBind.Thunk != nil {
					val, err = ev.forceThunk(inBind.Thunk)
				} else {
					val = inBind.Value
				}
				if err != nil {
					return nil, err
				}
				r, err := ev.applyCall2(f, nameVal, val, pos)
				if err != nil {
					return nil, err
				}
				return r, nil
			},
		}
		out.Attrs.Binds[sym] = &AttrBinding{Thunk: t}
	}
	return out, nil
}

func builtinAttrValues(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceAttrs(args[0], pos)
	if err != nil {
		return nil, err
	}
	var names []string
	for sym := range set.Attrs.Binds {
		names = append(names, ev.Intern.Lookup(sym))
	}
	stringsSort(names)
	out := make([]*Value, 0, len(names))
	for _, name := range names {
		sym := ev.Intern.Intern(name)
		b := set.Attrs.Binds[sym]
		if b == nil {
			continue
		}
		if b.Thunk != nil {
			out = append(out, Deferred(b.Thunk))
		} else {
			out = append(out, b.Value)
		}
	}
	return List(out...), nil
}

func builtinIsInt(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKInt), nil
}

func builtinIsFloat(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKFloat), nil
}

func builtinLessThan(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind == VKInt && b.Kind == VKInt {
		return Bool(a.Int < b.Int), nil
	}
	if a.Kind == VKFloat || b.Kind == VKFloat || a.Kind == VKInt || b.Kind == VKInt {
		var af, bf float64
		if a.Kind == VKInt {
			af = float64(a.Int)
		} else {
			af = a.Float
		}
		if b.Kind == VKInt {
			bf = float64(b.Int)
		} else {
			bf = b.Float
		}
		return Bool(af < bf), nil
	}
	return Bool(false), nil
}

func builtinAdd(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind == VKInt && b.Kind == VKInt {
		return Int(a.Int + b.Int), nil
	}
	return nil, evalErr(pos, "add expects integers")
}

func builtinSub(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind == VKInt && b.Kind == VKInt {
		return Int(a.Int - b.Int), nil
	}
	return nil, evalErr(pos, "sub expects integers")
}

func builtinIsString(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKString), nil
}

func builtinIsAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKAttrs || fv.Kind == VKDerivation), nil
}

func builtinIsFunction(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(ev.isFunctionValue(fv, pos)), nil
}

func (ev *Evaluator) isFunctionValue(fv *Value, pos parser.Pos) bool {
	switch fv.Kind {
	case VKLambda, VKBuiltin, VKPartialBuiltin:
		return true
	case VKAttrs:
		if !ev.hasFunctor(fv) {
			return false
		}
		sym := ev.Intern.Intern("__functor")
		b := fv.Attrs.Binds[sym]
		var functor *Value
		if b.Thunk != nil {
			v, err := ev.forceThunk(b.Thunk)
			if err != nil {
				return false
			}
			functor = v
		} else {
			functor = b.Value
		}
		fn, err := ev.applyCallable(functor, fv, pos)
		if err != nil {
			return false
		}
		fn, err = ev.forceValue(fn)
		if err != nil {
			return false
		}
		return ev.isFunctionValue(fn, pos)
	default:
		return false
	}
}

func builtinIsPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKPath), nil
}

func builtinTypeOf(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	var t string
	switch fv.Kind {
	case VKNull:
		t = "null"
	case VKBool:
		t = "bool"
	case VKInt:
		t = "int"
	case VKFloat:
		t = "float"
	case VKString:
		t = "string"
	case VKPath:
		t = "path"
	case VKList:
		t = "list"
	case VKAttrs, VKDerivation:
		t = "set"
	case VKLambda:
		t = "lambda"
	case VKBuiltin, VKPartialBuiltin:
		t = "function"
	default:
		t = "string"
	}
	return Str(t, pos), nil
}

func builtinSubstring(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 3); err != nil {
		return nil, err
	}
	startV, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	lenV, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	sv, err := ev.coerceToString(args[2], CoerceContext{Mode: CoerceString, Pos: pos})
	if err != nil {
		return nil, err
	}
	if startV.Kind != VKInt || lenV.Kind != VKInt {
		return nil, evalErr(pos, "substring start and length must be integers")
	}
	start := int(startV.Int)
	length := int(lenV.Int)
	runes := []rune(sv.String)
	if start < 0 {
		start = len(runes) + start
	}
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	var end int
	if length < 0 {
		// Nix: negative length means "until end of string" (removePrefix uses substring preLen (-1) str).
		end = len(runes)
	} else {
		end = start + length
		if end > len(runes) {
			end = len(runes)
		}
	}
	if end < start {
		end = start
	}
	return StrWithContext(string(runes[start:end]), copyStringContext(sv.Ctx), pos), nil
}

func builtinStringLength(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return Int(int64(len([]byte(s)))), nil
}

func builtinConcatStringsSep(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	sep, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	var ctx []StringContextElem
	cc := CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: false}
	for i, el := range list {
		if i > 0 {
			b.WriteString(sep)
		}
		sv, err := ev.coerceToString(el, cc)
		if err != nil {
			return nil, err
		}
		b.WriteString(sv.String)
		ctx = mergeStringContexts(ctx, sv.Ctx)
	}
	return StrWithContext(b.String(), ctx, pos), nil
}

func builtinParseDrvName(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	base := s
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if strings.HasSuffix(base, ".drv") {
		base = strings.TrimSuffix(base, ".drv")
	}
	name := base
	version := ""
	if i := strings.LastIndex(base, "-"); i > 0 {
		verPart := base[i+1:]
		if len(verPart) > 0 && verPart[0] >= '0' && verPart[0] <= '9' {
			name = base[:i]
			version = verPart
		}
	}
	out := Attrs(false)
	out.Attrs.Binds[ev.Intern.Intern("name")] = &AttrBinding{Value: Str(name, pos)}
	out.Attrs.Binds[ev.Intern.Intern("version")] = &AttrBinding{Value: Str(version, pos)}
	return out, nil
}

func builtinUnsafeDiscardStringContext(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	return Str(s, pos), nil
}

func builtinIsBool(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKBool), nil
}

func builtinBitAnd(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return builtinBitOp(ev, pos, args, func(a, b int64) int64 { return a & b })
}

func builtinBitOr(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return builtinBitOp(ev, pos, args, func(a, b int64) int64 { return a | b })
}

func builtinBitXor(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return builtinBitOp(ev, pos, args, func(a, b int64) int64 { return a ^ b })
}

func builtinBitOp(ev *Evaluator, pos parser.Pos, args []*Value, op func(int64, int64) int64) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind != VKInt || b.Kind != VKInt {
		return nil, evalErr(pos, "bitwise operation expects integers")
	}
	return Int(op(a.Int, b.Int)), nil
}

func builtinMul(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind == VKInt && b.Kind == VKInt {
		return Int(a.Int * b.Int), nil
	}
	return nil, evalErr(pos, "mul expects integers")
}

func builtinDiv(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	a, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	b, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if a.Kind == VKInt && b.Kind == VKInt {
		if b.Int == 0 {
			return nil, evalErr(pos, "division by zero")
		}
		return Int(a.Int / b.Int), nil
	}
	return nil, evalErr(pos, "div expects integers")
}

func (ev *Evaluator) forceStringOrPath(v *Value) (string, error) {
	fv, err := ev.forceValue(v)
	if err != nil {
		return "", err
	}
	switch fv.Kind {
	case VKString:
		return fv.String, nil
	case VKPath:
		return fv.Path, nil
	default:
		return "", fmt.Errorf("expected string or path, got %v", fv.Kind)
	}
}

func builtinBaseNameOf(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceStringOrPath(args[0])
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	s = strings.TrimSuffix(s, "/")
	if s == "" || s == "/" {
		return nil, evalErr(pos, "cannot take baseNameOf /")
	}
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	return Str(s, pos), nil
}

func builtinDirOf(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceStringOrPath(args[0])
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	if s == "" {
		return Str(".", pos), nil
	}
	s = strings.TrimSuffix(s, "/")
	i := strings.LastIndex(s, "/")
	if i < 0 {
		return Str(".", pos), nil
	}
	if i == 0 {
		return Str("/", pos), nil
	}
	return Str(s[:i], pos), nil
}

func builtinUnsafeGetAttrPos(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "unsafeGetAttrPos expects an attribute set")
	}
	sym := ev.Intern.Intern(name)
	b, ok := set.Attrs.Binds[sym]
	if !ok || b == nil {
		return Null(), nil
	}
	if b.Pos.File == "" && b.Pos.Line == 0 && b.Pos.Col == 0 {
		return Null(), nil
	}
	out := Attrs(false)
	out.Attrs.Binds[ev.Intern.Intern("file")] = &AttrBinding{Value: Str(b.Pos.File, pos)}
	out.Attrs.Binds[ev.Intern.Intern("line")] = &AttrBinding{Value: Int(int64(b.Pos.Line))}
	out.Attrs.Binds[ev.Intern.Intern("column")] = &AttrBinding{Value: Int(int64(b.Pos.Col))}
	return out, nil
}

func builtinGenericClosure(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	cfg, err := ev.forceAttrs(args[0], pos)
	if err != nil {
		return nil, err
	}
	if cfg.Kind != VKAttrs {
		return nil, evalErr(pos, "genericClosure expects an attribute set")
	}
	startSym := ev.Intern.Intern("startSet")
	opSym := ev.Intern.Intern("operator")
	keySym := ev.Intern.Intern("key")
	startBind := cfg.Attrs.Binds[startSym]
	if startBind == nil {
		return nil, evalErr(pos, "attribute 'startSet' required")
	}
	startSet, err := ev.attrBindingValue(startBind)
	if err != nil {
		return nil, err
	}
	startList, err := forceList(ev, pos, startSet)
	if err != nil {
		return nil, err
	}
	if len(startList) == 0 {
		return List(), nil
	}
	opBind := cfg.Attrs.Binds[opSym]
	if opBind == nil {
		return nil, evalErr(pos, "attribute 'operator' required")
	}
	op, err := ev.attrBindingValue(opBind)
	if err != nil {
		return nil, err
	}
	opFn, err := ev.forceValue(op)
	if err != nil {
		return nil, err
	}
	if opFn.Kind != VKLambda && opFn.Kind != VKBuiltin && opFn.Kind != VKPartialBuiltin {
		return nil, evalErr(pos, "operator must be a function")
	}

	var doneKeys []*Value
	keyDone := func(k *Value) (bool, error) {
		for _, dk := range doneKeys {
			eq, err := ev.equalValues(dk, k)
			if err != nil {
				return false, err
			}
			if eq {
				return true, nil
			}
		}
		return false, nil
	}

	workSet := append([]*Value{}, startList...)
	var res []*Value

	for len(workSet) > 0 {
		e := workSet[0]
		workSet = workSet[1:]

		eAttrs, err := ev.forceValue(e)
		if err != nil {
			return nil, err
		}
		if eAttrs.Kind != VKAttrs {
			return nil, evalErr(pos, "genericClosure element must be an attribute set")
		}
		keyBind := eAttrs.Attrs.Binds[keySym]
		if keyBind == nil {
			return nil, evalErr(pos, "attribute 'key' required")
		}
		keyVal, err := ev.attrBindingValue(keyBind)
		if err != nil {
			return nil, err
		}
		keyVal, err = ev.forceValue(keyVal)
		if err != nil {
			return nil, err
		}
		seen, err := keyDone(keyVal)
		if err != nil {
			return nil, err
		}
		if seen {
			continue
		}
		doneKeys = append(doneKeys, keyVal)
		res = append(res, e)

		newElems, err := ev.applyCall(opFn, e, pos)
		if err != nil {
			return nil, err
		}
		childList, err := forceList(ev, pos, newElems)
		if err != nil {
			return nil, err
		}
		for _, el := range childList {
			if _, err := ev.forceValue(el); err != nil {
				return nil, err
			}
			workSet = append(workSet, el)
		}
	}
	return List(res...), nil
}

func builtinFunctionArgs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	f, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	if f.Kind == VKAttrs {
		faSym := ev.Intern.Intern("__functionArgs")
		if b := f.Attrs.Binds[faSym]; b != nil {
			var meta *Value
			if b.Thunk != nil {
				meta, err = ev.forceThunk(b.Thunk)
			} else {
				meta = b.Value
			}
			if err != nil {
				return nil, err
			}
			if meta != nil && meta.Kind == VKAttrs {
				return meta, nil
			}
		}
		if ev.hasFunctor(f) {
			sym := ev.Intern.Intern("__functor")
			b := f.Attrs.Binds[sym]
			var functor *Value
			if b.Thunk != nil {
				functor, err = ev.forceThunk(b.Thunk)
			} else {
				functor = b.Value
			}
			if err != nil {
				return nil, err
			}
			fn, err := ev.applyCallable(functor, f, pos)
			if err != nil {
				return nil, err
			}
			return builtinFunctionArgs(ev, pos, []*Value{fn})
		}
	}
	out := Attrs(false)
	switch f.Kind {
	case VKLambda:
		if f.Lambda.Formals != nil {
			for _, p := range f.Lambda.Formals.Params {
				optional := p.Default != nil
				out.Attrs.Binds[ev.Intern.Intern(p.Name)] = &AttrBinding{Value: Bool(optional)}
			}
		} else if f.Lambda.SimpleArg != "" {
			out.Attrs.Binds[ev.Intern.Intern(f.Lambda.SimpleArg)] = &AttrBinding{Value: Bool(false)}
		}
	case VKBuiltin, VKPartialBuiltin:
		// no formal argument metadata
	default:
		return nil, evalErr(pos, "expected a function")
	}
	return out, nil
}

func builtinAddErrorContext(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	ctx, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	v := args[1]
	if v == nil {
		return Null(), nil
	}
	if ctx == "" {
		return v, nil
	}
	return ev.wrapErrorContext(ctx, v, pos), nil
}

func (ev *Evaluator) wrapErrorContext(ctx string, v *Value, pos parser.Pos) *Value {
	if v == nil {
		return Null()
	}
	if ctx == "" {
		return v
	}
	if v.Kind == VKDeferred && v.Deferred != nil {
		t := v.Deferred
		if t.errCtx != "" {
			ctx = ctx + " " + t.errCtx
		}
		nt := *t
		nt.errCtx = ctx
		return Deferred(&nt)
	}
	t := &Thunk{
		eval:   ev,
		pos:    pos,
		errCtx: ctx,
		lazy: func(ev *Evaluator) (*Value, error) {
			return ev.forceValue(v)
		},
	}
	return Deferred(t)
}
