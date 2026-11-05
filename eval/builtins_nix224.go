// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/connerohnesorge/goonix/data"
	"github.com/connerohnesorge/goonix/parser"
	"github.com/connerohnesorge/goonix/store"
	"github.com/nix-community/go-nix/pkg/nixhash"
)

func (ev *Evaluator) materializeTreeSource(sourceDir, name string) (*Value, error) {
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	logical, err := cfg.StorePathForFilesystemPath(sourceDir, name)
	if err != nil {
		return nil, err
	}
	phys := ev.physicalStorePath(logical)
	err = ev.Effects.RealiseStorePath(logical, func() error {
		if storePathMaterialised(phys) {
			return nil
		}
		if err := os.RemoveAll(phys); err != nil && !os.IsNotExist(err) {
			return err
		}
		return store.CopyPathToStore(sourceDir, phys)
	})
	if err != nil {
		return nil, err
	}
	return StrWithContext(logical, []StringContextElem{{StorePath: logical}}, parser.Pos{}), nil
}

func builtinGroupBy(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	if fv.Kind != VKLambda && fv.Kind != VKBuiltin && fv.Kind != VKPartialBuiltin {
		return nil, evalErr(pos, "groupBy expects a function")
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	binds := make(map[data.Symbol]*AttrBinding)
	for _, el := range list {
		keyV, err := ev.applyCall(fv, el, pos)
		if err != nil {
			return nil, err
		}
		key, err := ev.forceString(keyV)
		if err != nil {
			return nil, err
		}
		sym := ev.Intern.Intern(key)
		b, ok := binds[sym]
		if !ok {
			b = &AttrBinding{Value: List()}
			binds[sym] = b
		}
		b.Value.List = append(b.Value.List, el)
	}
	return &Value{Kind: VKAttrs, Attrs: &AttrSet{Binds: binds}}, nil
}

func builtinPartition(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	fv, err := ev.mustWHNF(args[0], pos)
	if err != nil {
		return nil, err
	}
	if fv.Kind != VKLambda && fv.Kind != VKBuiltin && fv.Kind != VKPartialBuiltin {
		return nil, evalErr(pos, "partition expects a function")
	}
	list, err := forceList(ev, pos, args[1])
	if err != nil {
		return nil, err
	}
	var right, wrong []*Value
	for _, el := range list {
		okV, err := ev.applyCall(fv, el, pos)
		if err != nil {
			return nil, err
		}
		ok, err := ev.forceBool(ev, okV, nil)
		if err != nil {
			return nil, err
		}
		if ok {
			right = append(right, el)
		} else {
			wrong = append(wrong, el)
		}
	}
	out := Attrs(false)
	out.Attrs.Binds[ev.Intern.Intern("right")] = &AttrBinding{Value: List(right...)}
	out.Attrs.Binds[ev.Intern.Intern("wrong")] = &AttrBinding{Value: List(wrong...)}
	return out, nil
}

func builtinConvertHash(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	set, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if set.Kind != VKAttrs {
		return nil, evalErr(pos, "convertHash expects attribute set")
	}
	hashStr, err := getAttrString(ev, set, "hash")
	if err != nil {
		return nil, err
	}
	toFmt, err := getAttrString(ev, set, "toHashFormat")
	if err != nil {
		return nil, err
	}
	hw, err := store.ParseHash(hashStr)
	if err != nil {
		return nil, err
	}
	var enc nixhash.Encoding
	switch toFmt {
	case "nix32", "base32":
		enc = nixhash.NixBase32
	case "hex", "base16":
		enc = nixhash.Base16
	case "sri":
		enc = nixhash.SRI
	default:
		return nil, evalErr(pos, "unknown hash format %q", toFmt)
	}
	return Str(hw.Format(enc, false), pos), nil
}

func builtinHashFile(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	algo, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	path, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	var h hash.Hash
	switch algo {
	case "md5":
		h = md5.New()
	case "sha1":
		h = sha1.New()
	case "sha256":
		h = sha256.New()
	case "sha512":
		h = sha512.New()
	default:
		return nil, evalErr(pos, "unsupported hash algorithm %q", algo)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(h, f); err != nil {
		return nil, err
	}
	return Str(hex.EncodeToString(h.Sum(nil)), pos), nil
}

func builtinFromTOML(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	v, err := parseTOMLDocument(ev, s)
	if err != nil {
		return nil, evalErr(pos, "fromTOML: %v", err)
	}
	return v, nil
}

func builtinReadFileType(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	p, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	info, err := ev.Effects.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return Str("unknown", pos), nil
		}
		return nil, err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		return Str("symlink", pos), nil
	case info.IsDir():
		return Str("directory", pos), nil
	case info.Mode().IsRegular():
		return Str("regular", pos), nil
	default:
		return Str("unknown", pos), nil
	}
}

func builtinScopedImport(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	scope, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	path, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	if (strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../")) && ev.curFile != "" {
		path = filepath.Join(filepath.Dir(ev.curFile), path)
	}
	path = resolveImportPath(path)
	return ev.importModuleInScope(path, scope)
}

func (ev *Evaluator) importModuleInScope(path string, scope *Value) (*Value, error) {
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
	if scope != nil && scope.Kind == VKAttrs {
		for sym, b := range scope.Attrs.Binds {
			if b.Thunk != nil {
				env.bindThunk(sym, b.Thunk)
			} else {
				env.bind(sym, b.Value)
			}
		}
	}
	return ev.evalExpr(expr, env)
}

func builtinTraceVerbose(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	if _, err := ev.forceString(args[0]); err != nil {
		return nil, err
	}
	return args[1], nil
}

func builtinBreak(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	return args[0], nil
}

func builtinCurrentTime(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if len(args) != 0 {
		return nil, evalErr(pos, "currentTime expects no arguments")
	}
	return Int(time.Now().Unix()), nil
}

func builtinToPath(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	s, err := ev.forceString(args[0])
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(s) {
		if ev.curFile != "" {
			s = filepath.Join(filepath.Dir(ev.curFile), s)
		}
	}
	return Path(s), nil
}

func builtinHasContextReal(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	sv, err := ev.coerceToString(args[0], CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: false})
	if err != nil {
		return nil, err
	}
	return Bool(len(sv.Ctx) > 0), nil
}

func builtinLangVersion6(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	return Int(6), nil
}

func builtinAddDrvOutputDependencies(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 2); err != nil {
		return nil, err
	}
	sv, err := ev.coerceToString(args[0], CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: false})
	if err != nil {
		return nil, err
	}
	if len(sv.Ctx) != 1 {
		return nil, evalErr(pos, "context of string %q must have exactly one element, but has %d", sv.String, len(sv.Ctx))
	}
	drvPath, err := ev.forceString(args[1])
	if err != nil {
		return nil, err
	}
	elem := sv.Ctx[0]
	if elem.Output == "" {
		return nil, evalErr(pos, "addDrvOutputDependencies expects a placeholder context element")
	}
	newCtx := []StringContextElem{{DrvPath: drvPath, Output: elem.Output}}
	return StrWithContext(sv.String, newCtx, pos), nil
}

func builtinUnsafeDiscardOutputDependency(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	sv, err := ev.coerceToString(args[0], CoerceContext{Mode: CoerceString, Pos: pos, CopyPaths: false})
	if err != nil {
		return nil, err
	}
	out := make([]StringContextElem, 0, len(sv.Ctx))
	for _, c := range sv.Ctx {
		if c.Output != "" {
			sp := c.StorePath
			if sp == "" {
				sp = sv.String
			}
			out = append(out, StringContextElem{StorePath: sp})
			continue
		}
		out = append(out, c)
	}
	return StrWithContext(sv.String, out, pos), nil
}

func fetchURLBytes(rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "file":
		p := u.Path
		if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") {
			p = p[1:]
		}
		return os.ReadFile(p)
	case "http", "https":
		resp, err := http.Get(rawURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %s", resp.Status)
		}
		return io.ReadAll(resp.Body)
	default:
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
}

func unpackTarball(data []byte, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	try := func(r io.Reader) error {
		tr := tar.NewReader(r)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			target := filepath.Join(dest, hdr.Name)
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0o755); err != nil {
					return err
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
				if err != nil {
					return err
				}
				if _, err := io.Copy(f, tr); err != nil {
					f.Close()
					return err
				}
				f.Close()
			}
		}
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err == nil {
		defer gr.Close()
		if err := try(gr); err == nil {
			return nil
		}
	}
	return try(bytes.NewReader(data))
}

func builtinFetchTarball(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	params, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	var rawURL, name, sha string
	switch params.Kind {
	case VKString:
		rawURL = params.String
		name = "source"
	case VKAttrs:
		rawURL, err = getAttrString(ev, params, "url")
		if err != nil {
			return nil, err
		}
		name, _ = getAttrString(ev, params, "name")
		if name == "" {
			name = "source"
		}
		sha, _ = getAttrString(ev, params, "sha256")
	default:
		return nil, evalErr(pos, "fetchTarball expects a string or an attribute set")
	}
	data, err := fetchURLBytes(rawURL)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "goonix-tar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	unpackDir := filepath.Join(tmp, "tree")
	if err := unpackTarball(data, unpackDir); err != nil {
		// Single-file archive: write bytes as lone file.
		if err := os.MkdirAll(unpackDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(unpackDir, "data"), data, 0o644); err != nil {
			return nil, err
		}
	}
	if sha != "" {
		if err := verifyTreeHash(unpackDir, sha); err != nil {
			return nil, err
		}
	}
	return ev.materializeTreeSource(unpackDir, name)
}

func verifyTreeHash(treeDir, hashStr string) error {
	wantB32, err := store.RecursivePathHash(treeDir)
	if err != nil {
		return err
	}
	wantParsed, err := store.ParseHash("sha256:" + wantB32)
	if err != nil {
		return err
	}
	gotParsed, err := store.ParseHash(hashStr)
	if err != nil {
		return err
	}
	if wantParsed.Format(nixhash.Base16, false) != gotParsed.Format(nixhash.Base16, false) {
		return fmt.Errorf("NAR hash mismatch in input, expected %s but got %s", gotParsed.Format(nixhash.SRI, true), wantParsed.Format(nixhash.SRI, true))
	}
	return nil
}

func builtinFetchGit(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	params, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if params.Kind != VKAttrs {
		return nil, evalErr(pos, "fetchGit expects attribute set")
	}
	rawURL, err := getAttrString(ev, params, "url")
	if err != nil {
		return nil, err
	}
	rev, err := getAttrString(ev, params, "rev")
	if err != nil {
		return nil, err
	}
	name, _ := getAttrString(ev, params, "name")
	if name == "" {
		name = "source"
	}
	dir, cleanup, err := gitExportTree(rawURL, rev)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return ev.materializeTreeSource(dir, name)
}

func builtinFetchTree(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := ev.requireExperimental(pos, "fetch-tree"); err != nil {
		return nil, err
	}
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	params, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if params.Kind != VKAttrs {
		return nil, evalErr(pos, "fetchTree expects attribute set")
	}
	typ, _ := getAttrString(ev, params, "type")
	switch typ {
	case "git":
		rev, err := getAttrString(ev, params, "rev")
		if err != nil {
			return nil, err
		}
		rawURL, err := getAttrString(ev, params, "url")
		if err != nil {
			return nil, err
		}
		dir, cleanup, err := gitExportTree(rawURL, rev)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		return ev.materializeTreeSource(dir, "source")
	case "path":
		p, err := getAttrString(ev, params, "path")
		if err != nil {
			return nil, err
		}
		if (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")) && ev.curFile != "" {
			p = filepath.Join(filepath.Dir(ev.curFile), p)
		}
		return ev.materializeTreeSource(p, "source")
	case "github":
		return nil, evalErr(pos, "fetchTree type %q not implemented", typ)
	default:
		return nil, evalErr(pos, "fetchTree type %q not implemented", typ)
	}
}

func (ev *Evaluator) filterSourceToStore(pos parser.Pos, pred *Value, src, name string) (*Value, error) {
	tmp, err := os.MkdirTemp("", "goonix-filter-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dest := filepath.Join(tmp, "out")
	if err := ev.copyFiltered(pos, pred, src, dest, true); err != nil {
		return nil, err
	}
	return ev.materializeTreeSource(dest, name)
}

func (ev *Evaluator) copyFiltered(pos parser.Pos, pred *Value, src, dest string, isRoot bool) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	typ := "unknown"
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		typ = "symlink"
	case info.IsDir():
		typ = "directory"
	case info.Mode().IsRegular():
		typ = "regular"
	}
	okV, err := ev.applyCall2(pred, Str(src, pos), Str(typ, pos), pos)
	if err != nil {
		return err
	}
	keep, err := ev.forceBool(ev, okV, nil)
	if err != nil {
		return err
	}
	if !keep && !isRoot {
		return nil
	}
	if info.IsDir() {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			if err := ev.copyFiltered(pos, pred, filepath.Join(src, ent.Name()), filepath.Join(dest, ent.Name()), false); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, info.Mode().Perm())
}

func builtinFetchMercurial(ev *Evaluator, pos parser.Pos, args []*Value) (*Value, error) {
	if err := arity(pos, args, 1); err != nil {
		return nil, err
	}
	params, err := ev.forceValue(args[0])
	if err != nil {
		return nil, err
	}
	if params.Kind != VKAttrs {
		return nil, evalErr(pos, "fetchMercurial expects attribute set")
	}
	rawURL, err := getAttrString(ev, params, "url")
	if err != nil {
		return nil, err
	}
	rev, _ := getAttrString(ev, params, "rev")
	name, _ := getAttrString(ev, params, "name")
	if name == "" {
		name = "source"
	}
	if _, err := exec.LookPath("hg"); err != nil {
		return nil, evalErr(pos, "fetchMercurial: hg not found on PATH")
	}
	tmp, err := os.MkdirTemp("", "goonix-hg-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dest := filepath.Join(tmp, "src")
	cmd := exec.Command("hg", "clone", rawURL, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("hg clone: %w\n%s", err, out)
	}
	if rev != "" {
		cmd := exec.Command("hg", "update", "-r", rev)
		cmd.Dir = dest
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("hg update: %w\n%s", err, out)
		}
	}
	return ev.materializeTreeSource(dest, name)
}
