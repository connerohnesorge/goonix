// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

type flakeRef struct {
	Type    string
	ID      string
	Owner   string
	Repo    string
	Rev     string
	Ref     string
	Dir     string
	Host    string
	NarHash string
	Path    string
}

func flakeIDPattern(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
		return false
	}
	for i := 1; i < len(s); i++ {
		c = s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

// parseIndirectFlakeRef parses Nix's indirect flake ids. A bare id that is
// not a flake id returns ok=false so the caller can try another form.
// fromURL is the flake: scheme, which reports an invalid id instead.
func parseIndirectFlakeRef(original, path string, fromURL bool) (flakeRef, bool, error) {
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" || !flakeIDPattern(parts[0]) {
		if fromURL && parts[0] != "" {
			return flakeRef{}, true, fmt.Errorf("'%s' is not a valid flake ID", parts[0])
		}
		return flakeRef{}, false, nil
	}
	out := flakeRef{Type: "indirect", ID: parts[0]}
	switch len(parts) {
	case 1:
	case 2:
		if isGitRev(parts[1]) {
			out.Rev = parts[1]
		} else if isGitRef(parts[1]) {
			out.Ref = parts[1]
		} else {
			return flakeRef{}, true, fmt.Errorf("in flake URL '%s', '%s' is not a commit hash or branch/tag name", original, parts[1])
		}
	case 3:
		if !isGitRef(parts[1]) {
			return flakeRef{}, true, fmt.Errorf("in flake URL '%s', '%s' is not a branch/tag name", original, parts[1])
		}
		if !isGitRev(parts[2]) {
			return flakeRef{}, true, fmt.Errorf("in flake URL '%s', '%s' is not a commit hash", original, parts[2])
		}
		out.Ref = parts[1]
		out.Rev = parts[2]
	default:
		return flakeRef{}, true, fmt.Errorf("GitHub URL '%s' is invalid", original)
	}
	return out, true, nil
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

func parseFlakeRefString(ref string) (flakeRef, error) {
	if ref == "" {
		return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
	}
	if strings.HasPrefix(ref, "/") {
		return flakeRef{Type: "path", Path: ref}, nil
	}
	if !strings.Contains(ref, ":") {
		if parsed, ok, err := parseIndirectFlakeRef(ref, ref, false); ok || err != nil {
			return parsed, err
		}
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
	case "github", "gitlab", "sourcehut":
		// github, gitlab, and sourcehut share Nix's GitArchiveInputScheme.
		// A sourcehut owner may contain '~'.
		return parseGitArchiveRef(scheme, rest, ref)
	case "flake":
		if strings.ContainsAny(rest, " \t") {
			return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
		}
		parsed, _, err := parseIndirectFlakeRef(ref, rest, true)
		return parsed, err
	default:
		return flakeRef{}, fmt.Errorf("flake reference '%s' is not an absolute path", ref)
	}
}

// parseGitArchiveRef parses owner/repo[/rev-or-ref] the way Nix 2.24.9's
// GitArchiveInputScheme does. A single extra segment that is 40 hex digits
// is a rev. More than one extra segment is joined with "/" and kept as a
// ref, so refs/heads/main stays one ref. Query keys rev, ref, host, narHash,
// and dir are kept; other query keys are dropped.
func parseGitArchiveRef(scheme, rest, original string) (flakeRef, error) {
	pathPart, query, _ := strings.Cut(rest, "?")
	parts := strings.Split(pathPart, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return flakeRef{}, fmt.Errorf("invalid %s flake reference '%s'", scheme, original)
	}
	out := flakeRef{Type: scheme, Owner: parts[0], Repo: parts[1]}
	switch {
	case len(parts) == 3 && parts[2] != "":
		if isGitRev(parts[2]) {
			out.Rev = parts[2]
		} else if isGitRef(parts[2]) {
			out.Ref = parts[2]
		} else {
			return flakeRef{}, fmt.Errorf("invalid %s flake reference '%s'", scheme, original)
		}
	case len(parts) > 3:
		joined := strings.Join(parts[2:], "/")
		if !isGitRef(joined) {
			return flakeRef{}, fmt.Errorf("invalid %s flake reference '%s'", scheme, original)
		}
		out.Ref = joined
	}
	q := decodeFlakeQuery(query)
	if rev := q["rev"]; rev != "" {
		if !isGitRev(rev) {
			return flakeRef{}, fmt.Errorf("invalid %s flake reference '%s'", scheme, original)
		}
		out.Rev = rev
	}
	if ref := q["ref"]; ref != "" {
		if out.Ref != "" {
			return flakeRef{}, fmt.Errorf("flake URL '%s' has multiple branch/tag names", original)
		}
		if !isGitRef(ref) {
			return flakeRef{}, fmt.Errorf("invalid %s flake reference '%s'", scheme, original)
		}
		out.Ref = ref
	}
	out.Host = q["host"]
	out.NarHash = q["narHash"]
	out.Dir = q["dir"]
	return out, nil
}

func isGitRef(s string) bool {
	if s == "" {
		return false
	}
	if !isGitRefFirst(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if !isGitRefRest(s[i]) {
			return false
		}
	}
	return true
}

func isGitRefFirst(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '@'
}

func isGitRefRest(c byte) bool {
	return isGitRefFirst(c) || c == '_' || c == '.' || c == '/' || c == '+' || c == '-'
}

func decodeFlakeQuery(q string) map[string]string {
	out := map[string]string{}
	if q == "" {
		return out
	}
	for _, kv := range strings.Split(q, "&") {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		dk, err := url.PathUnescape(k)
		if err != nil {
			dk = k
		}
		dv, err := url.PathUnescape(v)
		if err != nil {
			dv = v
		}
		out[dk] = dv
	}
	return out
}

// percentEncodeQuery matches Nix's percentEncode for query components.
// Unreserved characters and :@/? are left alone. '+' is not a space.
func percentEncodeQuery(s string) string {
	const keep = ":@/?"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("-._~", rune(c)) || strings.ContainsRune(keep, rune(c)) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

func (f flakeRef) String() string {
	switch f.Type {
	case "indirect":
		s := "flake:" + f.ID
		if f.Ref != "" {
			s += "/" + f.Ref
		}
		if f.Rev != "" {
			s += "/" + f.Rev
		}
		return s
	case "github", "gitlab", "sourcehut":
		s := f.Type + ":" + f.Owner + "/" + f.Repo
		switch {
		case f.Rev != "":
			s += "/" + f.Rev
		case f.Ref != "":
			s += "/" + f.Ref
		}
		q := map[string]string{}
		if f.Dir != "" {
			q["dir"] = f.Dir
		}
		if f.NarHash != "" {
			q["narHash"] = f.NarHash
		}
		if len(q) == 0 {
			return s
		}
		keys := make([]string, 0, len(q))
		for k := range q {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, percentEncodeQuery(k)+"="+percentEncodeQuery(q[k]))
		}
		return s + "?" + strings.Join(parts, "&")
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
	case "indirect":
		put("id", f.ID)
		if f.Rev != "" {
			put("rev", f.Rev)
		}
		if f.Ref != "" {
			put("ref", f.Ref)
		}
		if f.NarHash != "" {
			put("narHash", f.NarHash)
		}
	case "github", "gitlab", "sourcehut":
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
		if f.Host != "" {
			put("host", f.Host)
		}
		if f.NarHash != "" {
			put("narHash", f.NarHash)
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
	case "indirect":
		f.ID, err = getAttrString(ev, set, "id")
		if err != nil {
			return nil, err
		}
		if !flakeIDPattern(f.ID) {
			return nil, evalErr(pos, "'%s' is not a valid flake ID", f.ID)
		}
		if rev, err := getAttrString(ev, set, "rev"); err == nil && rev != "" {
			f.Rev = rev
		}
		if ref, err := getAttrString(ev, set, "ref"); err == nil {
			f.Ref = ref
		}
	case "github", "gitlab", "sourcehut":
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
		if host, err := getAttrString(ev, set, "host"); err == nil {
			f.Host = host
		}
		if narHash, err := getAttrString(ev, set, "narHash"); err == nil {
			f.NarHash = narHash
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
	if parsed.Type == "indirect" {
		resolved, err := lookupFlakeInput(flakeInputFromRef(parsed))
		if err != nil {
			return nil, evalErr(pos, "%s", err.Error())
		}
		parsed = flakeRefFromInput(resolved)
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
