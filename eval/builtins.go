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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
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
	register("filter", builtinFilter)
	register("foldl'", builtinFoldlPrime)
	register("attrNames", builtinAttrNames)
	register("getAttr", builtinGetAttr)
	register("hasAttr", builtinHasAttr)
	register("removeAttrs", builtinRemoveAttrs)
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
	register("filterSource", builtinFilterSourceStub)
	registerNullary("nixPath", builtinNixPath)
	register("findFile", builtinFindFile)
	registerNullary("storeDir", builtinStoreDir)
	registerNullary("nixVersion", builtinNixVersion)
	register("compareVersions", builtinCompareVersions)
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
	moduleVal, err := ev.evalExpr(expr, env)
	if err != nil {
		return nil, err
	}
	ev.importCache[path] = moduleVal
	return moduleVal, nil
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
	v, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	return Str(v.DisplayString(ev, ev.Intern), pos), nil
}

func builtinMap(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	list, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if list.Kind != VKList {
		return nil, evalErr(pos, "map expects a list")
	}
	out := make([]*Value, len(list.List))
	for i, el := range list.List {
		r, err := ev.applyCall(f, el, pos)
		if err != nil {
			return nil, err
		}
		out[i], err = ev.forceValue(r)
		if err != nil {
			return nil, err
		}
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
	pred, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	list, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if list.Kind != VKList {
		return nil, evalErr(pos, "filter expects a list")
	}
	var out []*Value
	for _, el := range list.List {
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

func builtinFoldlPrime(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 3); err != nil {
		return nil, err
	}
	op, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	acc := args[1]
	list, err := ev.forceValue(args[2])
	if err != nil {
		return nil, err
	}
	if list.Kind != VKList {
		return nil, evalErr(pos, "foldl' expects a list")
	}
	for _, el := range list.List {
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
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "attrNames expects attribute set")
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
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	name, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	sym := ev.Intern.Intern(name)
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "getAttr expects attribute set")
	}
	b, ok := set.Attrs.Binds[sym]
	if !ok {
		return nil, evalErr(pos, "attribute %q missing", name)
	}
	if b.Thunk != nil {
		return ev.forceThunk(b.Thunk)
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
	set, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return Bool(false), nil
	}
	sym := ev.Intern.Intern(name)
	_, ok := set.Attrs.Binds[sym]
	return Bool(ok), nil
}

func builtinRemoveAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	list, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs || list.Kind != VKList {
		return nil, evalErr(pos, "removeAttrs expects set and list")
	}
	out := Attrs(false)
removeLoop:
	for sym, bind := range set.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		for _, el := range list.List {
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

func builtinListToAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	list, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if list.Kind != VKList {
		return nil, evalErr(pos, "listToAttrs expects a list")
	}
	out := Attrs(false)
	for _, el := range list.List {
		item, err := ev.forceValue(el)
		if err != nil {
			return nil, err
		}
		if item.Kind != VKAttrs {
			return nil, evalErr(pos, "listToAttrs element must be attribute set")
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
		ns, err := ev.forceString(nb.Value)
		if err != nil {
			return nil, err
		}
		out.Attrs.Binds[ev.Intern.Intern(ns)] = &AttrBinding{Value: vb.Value}
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
	v, err := ev.deepForceValue(args[0])
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
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	entries, err := ev.Effects.ReadDir(p)
	if err != nil {
		return nil, err
	}
	elems := make([]*Value, len(entries))
	for i, e := range entries {
		elems[i] = Str(e.Name(), pos)
	}
	return List(elems...), nil
}

func builtinHashString(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) < 2 {
		return nil, evalErr(pos, "hashString expects at least 2 arguments")
	}
	algo, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	if algo != "sha256" {
		return nil, evalErr(pos, "unsupported hash algorithm %q", algo)
	}
	s, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	return Str(store.TextHash(s), pos), nil
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
	r, err := regexp.Compile(re)
	if err != nil {
		return nil, evalErr(pos, "invalid regex: %v", err)
	}
	m := r.FindStringSubmatch(s)
	if m == nil {
		return Null(), nil
	}
	if len(m) > 0 {
		m = m[1:]
	}
	elems := make([]*Value, len(m))
	for i, g := range m {
		elems[i] = Str(g, pos)
	}
	return List(elems...), nil
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
	parts := strings.Split(s, sep)
	elems := make([]*Value, len(parts))
	for i, p := range parts {
		elems[i] = Str(p, pos)
	}
	return List(elems...), nil
}

func builtinReplaceStrings(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 3); err != nil {
		return nil, err
	}
	from, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	to, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[2])
	if err != nil {
		return nil, err
	}
	return Str(strings.ReplaceAll(s, from, to), pos), nil
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
	return jsonToValue(raw, pos), nil
}

func jsonToValue(raw any, pos parser.Pos) *Value {
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
			elems[i] = jsonToValue(el, pos)
		}
		return List(elems...)
	case map[string]any:
		av := Attrs(false)
		intern := data.NewInterner()
		for k, v := range t {
			av.Attrs.Binds[intern.Intern(k)] = &AttrBinding{Value: jsonToValue(v, pos)}
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
	raw, err := valueToJSON(v)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return Str(string(b), pos), nil
}

func valueToJSON(v *Value) (any, error) {
	switch v.Kind {
	case VKNull:
		return nil, nil
	case VKBool:
		return v.Bool, nil
	case VKInt:
		return v.Int, nil
	case VKFloat:
		return v.Float, nil
	case VKString:
		return v.String, nil
	case VKList:
		out := make([]any, len(v.List))
		for i, el := range v.List {
			x, err := valueToJSON(el)
			if err != nil {
				return nil, err
			}
			out[i] = x
		}
		return out, nil
	case VKAttrs:
		m := make(map[string]any)
		for sym, bind := range v.Attrs.Binds {
			// sym is data.Symbol but we need intern lookup - skip for stub
			_ = sym
			_ = bind
		}
		return m, nil
	default:
		return fmt.Sprintf("<%v>", v.Kind), nil
	}
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
		return nil, evalErr(pos, "not a store path")
	}
	return Path(p), nil
}

func builtinPlaceholder(_ *Evaluator, pos parser.Pos, _ []*Value) (*Value, error) {
	return Str("unknown-tee", pos), nil
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
	return ev.derivationValueToAttrs(drv), nil
}

func builtinDerivation(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	// Lazy wrapper: for now same as strict.
	return builtinDerivationStrict(ev, pos, args)
}

func (ev *Evaluator) attrsToDerivation(attrs *Value) (*DerivationValue, error) {
	getS := func(name string) (string, error) {
		sym := ev.Intern.Intern(name)
		b, ok := attrs.Attrs.Binds[sym]
		if !ok {
			return "", fmt.Errorf("missing attribute %q", name)
		}
		var v *Value
		if b.Thunk != nil {
			var err error
			v, err = ev.forceThunk(b.Thunk)
			if err != nil {
				return "", err
			}
		} else {
			v = b.Value
		}
		return ev.forceString(v)
	}
	name, err := getS("name")
	if err != nil {
		return nil, err
	}
	system, err := getS("system")
	if err != nil {
		return nil, err
	}
	builder, err := getS("builder")
	if err != nil {
		return nil, err
	}
	drv := &DerivationValue{
		Name:    name,
		System:  system,
		Builder: builder,
		Env:     map[string]string{},
		Outputs: map[string]struct{}{"out": {}},
	}
	// args list
	argsSym := ev.Intern.Intern("args")
	if b, ok := attrs.Attrs.Binds[argsSym]; ok {
		var lv *Value
		if b.Thunk != nil {
			lv, err = ev.forceThunk(b.Thunk)
		} else {
			lv = b.Value
		}
		if err == nil && lv.Kind == VKList {
			for _, el := range lv.List {
				s, err := ev.forceString(el)
				if err != nil {
					return nil, err
				}
				drv.Args = append(drv.Args, s)
			}
		}
	}
	envSym := ev.Intern.Intern("env")
	if b, ok := attrs.Attrs.Binds[envSym]; ok {
		var evset *Value
		if b.Thunk != nil {
			evset, err = ev.forceThunk(b.Thunk)
		} else {
			evset = b.Value
		}
		if err == nil && evset.Kind == VKAttrs {
			for sym, bind := range evset.Attrs.Binds {
				key := ev.Intern.Lookup(sym)
				var val *Value
				if bind.Thunk != nil {
					val, err = ev.forceThunk(bind.Thunk)
				} else {
					val = bind.Value
				}
				if err != nil {
					return nil, err
				}
				s, err := ev.forceString(val)
				if err != nil {
					return nil, err
				}
				drv.Env[key] = s
			}
		}
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	drvPath, outPaths, err := computeDerivationPaths(cfg, drv)
	if err != nil {
		return nil, err
	}
	drv.DrvPath = drvPath
	drv.OutPaths = outPaths
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
	set("drvPath", Path(drv.DrvPath))
	if drv.OutPaths != nil {
		if p, ok := drv.OutPaths["out"]; ok {
			set("outPath", Path(p))
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
	return List(), nil
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
	content, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	storePath, err := cfg.StorePathForText(name, content)
	if err != nil {
		return nil, err
	}
	err = ev.Effects.RealiseStorePath(storePath, func() error {
		return store.WriteTextToStore(storePath, content)
	})
	if err != nil {
		return nil, err
	}
	return Path(storePath), nil
}

func builtinPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) < 1 {
		return nil, evalErr(pos, "path expects at least 1 argument")
	}
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	name := filepath.Base(p)
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	storePath, err := cfg.StorePathForFilesystemPath(p, name)
	if err != nil {
		return nil, err
	}
	err = ev.Effects.RealiseStorePath(storePath, func() error {
		return store.CopyPathToStore(p, storePath)
	})
	if err != nil {
		return nil, err
	}
	return Path(storePath), nil
}

func builtinFilterSourceStub(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) < 2 {
		return nil, evalErr(pos, "filterSource expects 2 arguments")
	}
	return args[len(args)-1], nil
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

func builtinNixVersion(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return Str(oracle.NixVersion, pos), nil
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
	if params.Kind != VKAttrs {
		return nil, evalErr(pos, "fetchurl expects attribute set")
	}
	url, err := getAttrString(ev, params, "url")
	if err != nil {
		return nil, err
	}
	hash, _ := getAttrString(ev, params, "sha256")
	name := filepath.Base(url)
	if n, err := getAttrString(ev, params, "name"); err == nil && n != "" {
		name = n
	}
	// Fixed-output derivation stub: return attrset mimicking fetchurl.nix result.
	drv := &DerivationValue{
		Name:    name,
		System:  currentSystemString(),
		Builder: "builtin:fetchurl",
		Args:    []string{url},
		Env: map[string]string{
			"url": url,
		},
		Outputs: map[string]struct{}{"out": {}},
	}
	if hash != "" {
		drv.Env["outputHash"] = hash
		drv.Env["outputHashAlgo"] = "sha256"
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	drvPath, outPaths, err := computeDerivationPaths(cfg, drv)
	if err != nil {
		return nil, err
	}
	drv.DrvPath = drvPath
	drv.OutPaths = outPaths
	return ev.derivationValueToAttrs(drv), nil
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
	fv, err := ev.forceValue(v)
	if err != nil {
		return nil, err
	}
	if fv.Kind != VKList {
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
	fv, err := ev.forceValue(args[0])
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
	pred, err := ev.forceValue(args[0])
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
	pred, err := ev.forceValue(args[0])
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
	f, err := ev.forceValue(args[0])
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
		r, err := ev.applyCall(f, Int(i), pos)
		if err != nil {
			return nil, err
		}
		out[i], err = ev.forceValue(r)
		if err != nil {
			return nil, err
		}
	}
	return List(out...), nil
}

func builtinMapAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	f, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[1])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "mapAttrs expects attribute set")
	}
	out := Attrs(false)
	for sym, bind := range set.Attrs.Binds {
		name := ev.Intern.Lookup(sym)
		var val *Value
		if bind.Thunk != nil {
			val, err = ev.forceThunk(bind.Thunk)
		} else {
			val = bind.Value
		}
		if err != nil {
			return nil, err
		}
		nameVal := Str(name, pos)
		r, err := ev.applyCall2(f, nameVal, val, pos)
		if err != nil {
			return nil, err
		}
		rv, err := ev.forceValue(r)
		if err != nil {
			return nil, err
		}
		out.Attrs.Binds[sym] = &AttrBinding{Value: rv}
	}
	return out, nil
}

func builtinAttrValues(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "attrValues expects attribute set")
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
			v, err := ev.forceThunk(b.Thunk)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
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
	fv, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKInt), nil
}

func builtinIsFloat(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.forceValue(args[0])
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
	fv, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKString), nil
}

func builtinIsAttrs(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKAttrs || fv.Kind == VKDerivation), nil
}

func builtinIsPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	return Bool(fv.Kind == VKPath), nil
}

func builtinTypeOf(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	fv, err := ev.forceValue(args[0])
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
	s, err := ev.forceString(args[2])
	if err != nil {
		return nil, err
	}
	if startV.Kind != VKInt || lenV.Kind != VKInt {
		return nil, evalErr(pos, "substring start and length must be integers")
	}
	start := int(startV.Int)
	length := int(lenV.Int)
	runes := []rune(s)
	if start < 0 {
		start = len(runes) + start
	}
	if start < 0 {
		start = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	end := start + length
	if end > len(runes) {
		end = len(runes)
	}
	if end < start {
		end = start
	}
	return Str(string(runes[start:end]), pos), nil
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
	var parts []string
	for _, el := range list {
		s, err := ev.forceString(el)
		if err != nil {
			return nil, err
		}
		parts = append(parts, s)
	}
	return Str(strings.Join(parts, sep), pos), nil
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
