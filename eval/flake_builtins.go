// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

type flakeRef struct {
	Type  string
	Owner string
	Repo  string
	Rev   string
	Ref   string
	Dir   string
	Path  string
}

func isGitRev(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

func queryParam(query, key string) string {
	for _, kv := range strings.Split(query, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}

func parseFlakeRefString(ref string) (flakeRef, error) {
	if ref == "" {
		return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
	}
	if strings.HasPrefix(ref, "/") {
		return flakeRef{Type: "path", Path: ref}, nil
	}
	scheme, rest, ok := strings.Cut(ref, ":")
	if !ok || scheme == "" || strings.Contains(scheme, "/") {
		return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
	}
	switch scheme {
	case "path":
		if rest == "" || !strings.HasPrefix(rest, "/") {
			return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
		}
		return flakeRef{Type: "path", Path: rest}, nil
	case "github":
		pathPart, query, _ := strings.Cut(rest, "?")
		parts := strings.Split(pathPart, "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return flakeRef{}, fmt.Errorf("invalid GitHub flake reference '%s'", ref)
		}
		out := flakeRef{Type: "github", Owner: parts[0], Repo: parts[1]}
		if len(parts) >= 3 && parts[2] != "" {
			// A 40-digit sha1 is a rev. Anything else, including a branch
			// such as nixos-24.05, is a ref.
			if isGitRev(parts[2]) {
				out.Rev = parts[2]
			} else {
				out.Ref = parts[2]
			}
		}
		if dir := queryParam(query, "dir"); dir != "" {
			out.Dir = dir
		}
		return out, nil
	default:
		return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
	}
}

func (f flakeRef) String() string {
	switch f.Type {
	case "github":
		s := "github:" + f.Owner + "/" + f.Repo
		switch {
		case f.Rev != "":
			s += "/" + f.Rev
		case f.Ref != "":
			s += "/" + f.Ref
		}
		if f.Dir != "" {
			s += "?dir=" + f.Dir
		}
		return s
	case "path":
		return "path:" + f.Path
	default:
		return ""
	}
}

func flakeRefAttrs(ev *Evaluator, f flakeRef, pos parser.Pos) *Value {
	set := Attrs(false)
	put := func(k, v string) {
		set.Attrs.Binds[ev.Intern.Intern(k)] = &AttrBinding{Value: Str(v, pos)}
	}
	put("type", f.Type)
	switch f.Type {
	case "github":
		put("owner", f.Owner)
		put("repo", f.Repo)
		if f.Rev != "" {
			put("rev", f.Rev)
		}
		if f.Ref != "" {
			put("ref", f.Ref)
		}
		if f.Dir != "" {
			put("dir", f.Dir)
		}
	case "path":
		put("path", f.Path)
	}
	return set
}

func builtinParseFlakeRef(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := ev.requireExperimental(pos, "flakes"); err != nil {
		return nil, err
	}
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	ref, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	parsed, err := parseFlakeRefString(ref)
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	return flakeRefAttrs(ev, parsed, pos), nil
}

func builtinFlakeRefToString(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := ev.requireExperimental(pos, "flakes"); err != nil {
		return nil, err
	}
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
	typ, err := getAttrString(ev, set, "type")
	if err != nil {
		return nil, err
	}
	f := flakeRef{Type: typ}
	switch typ {
	case "github":
		f.Owner, err = getAttrString(ev, set, "owner")
		if err != nil {
			return nil, err
		}
		f.Repo, err = getAttrString(ev, set, "repo")
		if err != nil {
			return nil, err
		}
		if rev, err := getAttrString(ev, set, "rev"); err == nil && rev != "" {
			if !isGitRev(rev) {
				return nil, evalErr(pos, "hash '%s' has wrong length for hash algorithm 'sha1'", rev)
			}
			f.Rev = rev
		}
		if ref, err := getAttrString(ev, set, "ref"); err == nil {
			f.Ref = ref
		}
		if dir, err := getAttrString(ev, set, "dir"); err == nil {
			f.Dir = dir
		}
	case "path":
		f.Path, err = getAttrString(ev, set, "path")
		if err != nil {
			return nil, err
		}
	default:
		return nil, evalErr(pos, "unknown flake reference type '%s'", typ)
	}
	return Str(f.String(), pos), nil
}

func builtinGetFlake(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := ev.requireExperimental(pos, "flakes"); err != nil {
		return nil, err
	}
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	ref, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	parsed, err := parseFlakeRefString(ref)
	if err != nil {
		return nil, evalErr(pos, "%s", err.Error())
	}
	dir := parsed.Path
	var cleanup func()
	if parsed.Type == "github" {
		// Resolve via the GitHub commits API and a bounded tarball download.
		// An unlocked github: ref must not start a git clone of HEAD.
		var err error
		revOrRef := parsed.Rev
		if revOrRef == "" {
			revOrRef = parsed.Ref
		}
		dir, cleanup, _, _, err = fetchGitHubTree(parsed.Owner, parsed.Repo, revOrRef)
		if err != nil {
			return nil, evalErr(pos, "%s", err.Error())
		}
		defer cleanup()
	} else if parsed.Type != "path" {
		return nil, evalErr(pos, "flake reference '%s' is not an absolute path", ref)
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			// Nix 2.24.9: path '/missing' does not exist
			return nil, evalErr(pos, "path '%s' does not exist", dir)
		}
		return nil, err
	}
	flakeNix := filepath.Join(dir, "flake.nix")
	if _, err := os.Stat(flakeNix); err != nil {
		if os.IsNotExist(err) {
			return nil, evalErr(pos, "path '%s' does not exist", flakeNix)
		}
		return nil, err
	}
	return ev.evalPathFlake(pos, dir, flakeNix)
}

func (ev *Evaluator) evalPathFlake(pos parser.Pos, dir, flakeNix string) (*Value, error) {
	src, err := ev.materializeTreeSource(dir, "source")
	if err != nil {
		return nil, err
	}
	outPath := src.String
	narHash, err := narSRI(dir)
	if err != nil {
		return nil, err
	}
	mod, err := ev.importModule(flakeNix)
	if err != nil {
		return nil, err
	}
	mod, err = ev.forceValue(mod)
	if err != nil {
		return nil, err
	}
	if mod.Kind != VKAttrs {
		return nil, evalErr(pos, "flake.nix must evaluate to an attribute set")
	}
	outputsVal, ok, err := attrValue(ev, mod, "outputs")
	if err != nil || !ok {
		return nil, evalErr(pos, "flake.nix missing outputs")
	}
	self := Attrs(false)
	self.Attrs.Binds[ev.Intern.Intern("outPath")] = &AttrBinding{Value: StrWithContext(outPath, []StringContextElem{{StorePath: outPath}}, pos)}
	self.Attrs.Binds[ev.Intern.Intern("narHash")] = &AttrBinding{Value: Str(narHash, pos)}
	self.Attrs.Binds[ev.Intern.Intern("_type")] = &AttrBinding{Value: Str("flake", pos)}
	arg := Attrs(false)
	arg.Attrs.Binds[ev.Intern.Intern("self")] = &AttrBinding{Value: self}
	produced, err := ev.applyCall(outputsVal, arg, pos)
	if err != nil {
		return nil, err
	}
	produced, err = ev.forceValue(produced)
	if err != nil {
		return nil, err
	}
	flake := Attrs(false)
	flake.Attrs.Binds[ev.Intern.Intern("outPath")] = self.Attrs.Binds[ev.Intern.Intern("outPath")]
	flake.Attrs.Binds[ev.Intern.Intern("narHash")] = self.Attrs.Binds[ev.Intern.Intern("narHash")]
	flake.Attrs.Binds[ev.Intern.Intern("_type")] = self.Attrs.Binds[ev.Intern.Intern("_type")]
	flake.Attrs.Binds[ev.Intern.Intern("outputs")] = &AttrBinding{Value: produced}
	if produced.Kind == VKAttrs {
		for sym, b := range produced.Attrs.Binds {
			flake.Attrs.Binds[sym] = b
		}
	}
	return flake, nil
}

func attrValue(ev *Evaluator, set *Value, name string) (*Value, bool, error) {
	sym := ev.Intern.Intern(name)
	b, ok := set.Attrs.Binds[sym]
	if !ok {
		return nil, false, nil
	}
	if b.Thunk != nil {
		v, err := ev.forceThunk(b.Thunk)
		return v, true, err
	}
	return b.Value, true, nil
}

func narSRI(dir string) (string, error) {
	b32, err := store.RecursivePathHash(dir)
	if err != nil {
		return "", err
	}
	raw, err := nixbase32.DecodeString(b32)
	if err != nil {
		return "", err
	}
	h, err := nixhash.NewHash(nixhash.SHA256, raw)
	if err != nil {
		return "", err
	}
	return h.Format(nixhash.SRI, true), nil
}
