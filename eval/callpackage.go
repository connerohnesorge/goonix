// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"strings"

	"github.com/connerohnesorge/goonix/parser"
)

// nixpkgs lib.customisation.callPackageWith autoArgs: fn: args: … treats
// (callPackageWith auto) scopeAttrs as callPackageWith (auto // scopeAttrs),
// not as binding fn = scopeAttrs. Same for newScope = extra: callPackageWith (auto // extra).

func (ev *Evaluator) tryMergeCallPackageWithAuto(l *LambdaValue, arg *Value, pos parser.Pos) (*Value, bool, error) {
	if l == nil || l.SimpleArg != "fn" {
		return nil, false, nil
	}
	autoSym := ev.Intern.Intern("autoArgs")
	b, ok := l.Env.lookup(autoSym)
	if !ok || b == nil {
		return nil, false, nil
	}
	autoVal, err := ev.attrBindingValue(b)
	if err != nil {
		return nil, false, err
	}
	argFV, err := ev.mustWHNF(arg, pos)
	if err != nil {
		return nil, false, err
	}
	if !ev.isCallPackageAutoMergeArg(argFV, pos) {
		return nil, false, nil
	}
	merged := mergeAttrBindMaps(autoVal, argFV)
	// Keep explicit autoArgs (e.g. fetchurl stubs) when spliced self also defines same names.
	merged = mergeAttrBindMaps(merged, autoVal)
	root := ev.callPackageWithRootLambda(l.Env)
	if root == nil {
		return nil, false, nil
	}
	out, err := ev.applyCallable(&Value{Kind: VKLambda, Lambda: root}, merged, pos)
	return out, true, err
}

func (ev *Evaluator) isCallPackageAutoMergeArg(v *Value, pos parser.Pos) bool {
	if v == nil {
		return false
	}
	if ev.isFunctionValue(v, pos) {
		return false
	}
	if v.Kind == VKPath {
		return false
	}
	if v.Kind == VKString {
		s := v.String
		return strings.HasPrefix(s, "/") || strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
			strings.HasSuffix(s, ".nix")
	}
	return v.Kind == VKAttrs
}

func (ev *Evaluator) callPackageWithRootLambda(env *Env) *LambdaValue {
	autoSym := ev.Intern.Intern("autoArgs")
	for cur := env; cur != nil; cur = cur.Parent {
		if cur.Applying != nil && cur.Applying.Kind == VKLambda && cur.Applying.Lambda != nil {
			l := cur.Applying.Lambda
			if l.SimpleArg == "autoArgs" && l.Formals == nil {
				return l
			}
		}
		if _, ok := cur.Vars[autoSym]; ok && cur.Applying != nil && cur.Applying.Kind == VKLambda {
			l := cur.Applying.Lambda
			if l != nil && l.SimpleArg == "autoArgs" {
				return l
			}
		}
	}
	return nil
}

func (ev *Evaluator) withCallPackageBinding(w *Value) (*AttrBinding, bool, error) {
	sym := ev.Intern.Intern("callPackage")
	pos := parser.Pos{}
	tryBind := func(v *Value) (*AttrBinding, bool, error) {
		if v == nil || v.Kind != VKAttrs || v.Attrs == nil {
			return nil, false, nil
		}
		b := v.Attrs.Binds[sym]
		if b == nil {
			return nil, false, nil
		}
		val, err := ev.attrBindingValue(b)
		if err != nil {
			return nil, false, err
		}
		fv, err := ev.mustWHNF(val, pos)
		if err != nil {
			return nil, false, err
		}
		if !ev.isFunctionValue(fv, pos) {
			return nil, false, nil
		}
		return b, true, nil
	}
	if w.Attrs != nil && w.Attrs.MergeLHS != nil {
		if b, ok, err := tryBind(&Value{Kind: VKAttrs, Attrs: w.Attrs.MergeLHS}); ok || err != nil {
			return b, ok, err
		}
	}
	if w.Attrs != nil && w.Attrs.UpdateRHS != nil {
		if b, ok, err := tryBind(&Value{Kind: VKAttrs, Attrs: w.Attrs.UpdateRHS}); ok || err != nil {
			return b, ok, err
		}
	}
	return tryBind(w)
}

func (ev *Evaluator) systemPredicatesFromSystemField(v *Value, name string) (*Value, bool) {
	if v == nil || v.Kind != VKAttrs || v.Attrs == nil {
		return nil, false
	}
	sysB := v.Attrs.Binds[ev.Intern.Intern("system")]
	if sysB == nil {
		return nil, false
	}
	sysV, err := ev.attrBindingValue(sysB)
	if err != nil || sysV == nil || sysV.Kind != VKString {
		return nil, false
	}
	parts := strings.Split(sysV.String, "-")
	if len(parts) < 2 {
		return nil, false
	}
	kernel := parts[len(parts)-1]
	switch name {
	case "isLinux":
		return Bool(kernel == "linux"), true
	case "isDarwin":
		return Bool(kernel == "darwin"), true
	case "isFreeBSD":
		return Bool(kernel == "freebsd"), true
	case "isOpenBSD":
		return Bool(kernel == "openbsd"), true
	case "isNetBSD":
		return Bool(kernel == "netbsd"), true
	case "isSunOS", "isSolaris":
		return Bool(kernel == "solaris"), true
	case "isCygwin":
		return Bool(kernel == "cygwin"), true
	case "libc":
		if kernel == "linux" {
			return Str("glibc", parser.Pos{}), true
		}
		return nil, false
	default:
		return nil, false
	}
}
