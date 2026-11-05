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
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
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

// treeMeta is the backend-specific half of Nix 2.24.9 emitTreeAttrs.
// hasLast is set for path, git, and tarball inputs that have a mtime.
// revCount is set only when a git input is not shallow.
type treeMeta struct {
	hasLast      bool
	lastModified int64
	rev          string
	revCount     *int64
	submodules   *bool
}

func (ev *Evaluator) emitTreeAttrs(pos parser.Pos, outPath, narHash string, meta treeMeta) *Value {
	av := Attrs(false)
	av.Attrs.Binds[ev.Intern.Intern("outPath")] = &AttrBinding{Value: StrWithContext(outPath, []StringContextElem{{StorePath: outPath}}, pos)}
	av.Attrs.Binds[ev.Intern.Intern("narHash")] = &AttrBinding{Value: Str(narHash, pos)}
	if meta.hasLast {
		av.Attrs.Binds[ev.Intern.Intern("lastModified")] = &AttrBinding{Value: Int(meta.lastModified)}
		av.Attrs.Binds[ev.Intern.Intern("lastModifiedDate")] = &AttrBinding{Value: Str(time.Unix(meta.lastModified, 0).UTC().Format("20060102150405"), pos)}
	}
	if meta.rev != "" {
		short := meta.rev
		if len(short) > 7 {
			short = short[:7]
		}
		av.Attrs.Binds[ev.Intern.Intern("rev")] = &AttrBinding{Value: Str(meta.rev, pos)}
		av.Attrs.Binds[ev.Intern.Intern("shortRev")] = &AttrBinding{Value: Str(short, pos)}
	}
	if meta.revCount != nil {
		av.Attrs.Binds[ev.Intern.Intern("revCount")] = &AttrBinding{Value: Int(*meta.revCount)}
	}
	if meta.submodules != nil {
		av.Attrs.Binds[ev.Intern.Intern("submodules")] = &AttrBinding{Value: Bool(*meta.submodules)}
	}
	return av
}

func (ev *Evaluator) treeFromDir(pos parser.Pos, dir, name string, meta treeMeta, computeLast bool) (*Value, error) {
	src, err := ev.materializeTreeSource(dir, name)
	if err != nil {
		return nil, err
	}
	nar, err := narSRI(dir)
	if err != nil {
		return nil, err
	}
	if computeLast {
		m, err := treeMaxMtime(dir)
		if err != nil {
			return nil, err
		}
		meta.hasLast = true
		meta.lastModified = m
	}
	return ev.emitTreeAttrs(pos, src.String, nar, meta), nil
}

func treeMaxMtime(root string) (int64, error) {
	var max int64
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		st, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if mt := st.ModTime().Unix(); mt > max {
			max = mt
		}
		return nil
	})
	return max, err
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
	var algo *nixhash.Algorithm
	algoName := ""
	if name, err := getAttrString(ev, set, "hashAlgo"); err == nil && name != "" {
		a, aerr := nixhash.ParseAlgorithm(name)
		if aerr != nil {
			return nil, evalErr(pos, "unknown hash algorithm '%s'", name)
		}
		algo = &a
		algoName = name
	}
	hw, err := nixhash.ParseAny(hashStr, algo)
	if err != nil {
		if algoName != "" {
			return nil, evalErr(pos, "hash '%s' has wrong length for hash algorithm '%s'", hashStr, algoName)
		}
		return nil, evalErr(pos, "%s", err.Error())
	}
	var enc nixhash.Encoding
	switch toFmt {
	case "nix32", "base32":
		enc = nixhash.NixBase32
	case "hex", "base16":
		enc = nixhash.Base16
	case "base64":
		enc = nixhash.Base64
	case "sri":
		enc = nixhash.SRI
	default:
		return nil, evalErr(pos, "unknown hash format '%s'", toFmt)
	}
	// Nix includes the algorithm prefix only for SRI.
	return Str(hw.Format(enc, enc == nixhash.SRI), pos), nil
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
		return nil, evalErr(pos, "unknown hash algorithm '%s'", algo)
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
		return nil, evalErr(pos, "while parsing TOML: %s", err.Error())
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
	info, err := os.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, evalErr(pos, "path '%s' does not exist", p)
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
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			// Nix 2.24.9's curl file:// error, including the CURLE code.
			return nil, fmt.Errorf("unable to download '%s': Couldn't read a file:// file (37)", rawURL)
		}
		return b, nil
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
		return nil, fmt.Errorf("URL scheme '%s' is not recognised", u.Scheme)
	}
}

func unpackTarball(data []byte, dest string) (int64, error) {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, err
	}
	var maxMtime int64
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
			if mt := hdr.ModTime.Unix(); mt > maxMtime {
				maxMtime = mt
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
			case tar.TypeSymlink:
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				_ = os.Remove(target)
				if err := os.Symlink(hdr.Linkname, target); err != nil {
					return err
				}
			}
		}
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err == nil {
		defer gr.Close()
		if err := try(gr); err == nil {
			return maxMtime, nil
		}
		maxMtime = 0
	}
	if err := try(bytes.NewReader(data)); err != nil {
		return 0, err
	}
	return maxMtime, nil
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
		return nil, archiveDownloadError(err)
	}
	tmp, err := os.MkdirTemp("", "goonix-tar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	unpackDir := filepath.Join(tmp, "tree")
	if _, err := unpackTarball(data, unpackDir); err != nil {
		// Single-file archive: write bytes as lone file.
		if err := os.MkdirAll(unpackDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(unpackDir, "data"), data, 0o644); err != nil {
			return nil, err
		}
	} else {
		unpackDir = stripSingleDir(unpackDir)
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
		if ev.xp == nil || !ev.xp["flakes"] {
			return nil, evalErr(pos, "passing a string argument to 'fetchTree' requires the 'flakes' experimental feature")
		}
		raw, err := ev.forceString(params)
		if err != nil {
			return nil, err
		}
		return ev.fetchTreeURL(pos, raw)
	}
	typ, err := getAttrString(ev, params, "type")
	if err != nil {
		return nil, evalErr(pos, "attribute 'type' is missing in call to 'fetchTree'")
	}
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
		shallow := true
		if b, ok, err := getAttrBool(ev, params, "shallow"); err != nil {
			return nil, err
		} else if ok {
			shallow = b
		}
		submodules := false
		if b, ok, err := getAttrBool(ev, params, "submodules"); err != nil {
			return nil, err
		} else if ok {
			submodules = b
		}
		dir, cleanup, meta, err := gitExportTreeMeta(rawURL, rev, !shallow)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		meta.submodules = &submodules
		return ev.treeFromDir(pos, dir, "source", meta, false)
	case "path":
		p, err := getAttrString(ev, params, "path")
		if err != nil {
			return nil, err
		}
		if (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")) && ev.curFile != "" {
			p = filepath.Join(filepath.Dir(ev.curFile), p)
		}
		return ev.treeFromDir(pos, p, "source", treeMeta{}, true)
	case "file":
		rawURL, err := getAttrString(ev, params, "url")
		if err != nil {
			return nil, err
		}
		return ev.fetchTreeFile(rawURL)
	case "tarball":
		rawURL, err := getAttrString(ev, params, "url")
		if err != nil {
			return nil, err
		}
		return ev.fetchTreeTarball(rawURL, true)
	case "github":
		owner, err := getAttrString(ev, params, "owner")
		if err != nil {
			return nil, err
		}
		repo, err := getAttrString(ev, params, "repo")
		if err != nil {
			return nil, err
		}
		rev, _ := getAttrString(ev, params, "rev")
		return ev.fetchTreeGitHub(owner, repo, rev)
	case "gitlab":
		owner, err := getAttrString(ev, params, "owner")
		if err != nil {
			return nil, err
		}
		repo, err := getAttrString(ev, params, "repo")
		if err != nil {
			return nil, err
		}
		rev, _ := getAttrString(ev, params, "rev")
		return ev.fetchTreeGitLab(owner, repo, rev)
	case "sourcehut":
		owner, err := getAttrString(ev, params, "owner")
		if err != nil {
			return nil, err
		}
		repo, err := getAttrString(ev, params, "repo")
		if err != nil {
			return nil, err
		}
		rev, _ := getAttrString(ev, params, "rev")
		return ev.fetchTreeSourceHut(owner, repo, rev)
	case "indirect":
		id, _ := getAttrString(ev, params, "id")
		return nil, evalErr(pos, "cannot find flake 'flake:%s' in the flake registries", id)
	default:
		// Nix 2.24 prints the input attrs. A single type attribute is the common case.
		return nil, evalErr(pos, "cannot show unsupported input '{\"type\":%q}'", typ)
	}
}

func downloadChecked(rawURL string) ([]byte, error) {
	// git.sr.ht answers curl (HTTP/1.1) with 404 and does not answer an
	// HTTP/2 Go client. Nix's fetch reports that 404.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false
	transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	client := &http.Client{Timeout: 20 * time.Second, Transport: transport}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("unable to download '%s': %s", rawURL, err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to download '%s': HTTP error %d\n\nresponse body:\n\n%s", rawURL, resp.StatusCode, body)
	}
	return body, nil
}

func (ev *Evaluator) fetchTreeURL(pos parser.Pos, raw string) (*Value, error) {
	switch {
	case strings.HasPrefix(raw, "github:"):
		parts := strings.Split(strings.TrimPrefix(raw, "github:"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
			return nil, evalErr(pos, "URL '%s' is invalid", raw)
		}
		rev := ""
		if len(parts) >= 3 {
			rev = parts[2]
		}
		return ev.fetchTreeGitHub(parts[0], parts[1], rev)
	case strings.HasPrefix(raw, "gitlab:"):
		parts := strings.Split(strings.TrimPrefix(raw, "gitlab:"), "/")
		if len(parts) < 2 {
			return nil, evalErr(pos, "URL '%s' is invalid", raw)
		}
		return ev.fetchTreeGitLab(parts[0], parts[1], "")
	case strings.HasPrefix(raw, "sourcehut:"):
		parts := strings.Split(strings.TrimPrefix(raw, "sourcehut:"), "/")
		if len(parts) < 2 {
			return nil, evalErr(pos, "URL '%s' is invalid", raw)
		}
		return ev.fetchTreeSourceHut(parts[0], parts[1], "")
	case strings.HasPrefix(raw, "flake:"):
		return nil, evalErr(pos, "cannot find flake '%s' in the flake registries", raw)
	default:
		return ev.fetchTreeTarball(raw, true)
	}
}

func (ev *Evaluator) fetchTreeFile(rawURL string) (*Value, error) {
	data, err := fetchURLBytes(rawURL)
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp("", "goonix-file-*")
	if err != nil {
		return nil, err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return nil, err
	}
	tmp.Close()
	defer os.Remove(name)
	// fetchTree re-ingests the downloaded file as a NAR ("source"), so the
	// outPath is the recursive path, not the flat download path.
	return ev.treeFromDir(parser.Pos{}, name, "source", treeMeta{}, false)
}

func archiveDownloadError(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "Couldn't read a file:// file") {
		return fmt.Errorf("Failed to open archive (Source threw exception: error: %s)", err.Error())
	}
	return err
}

func (ev *Evaluator) fetchTreeTarball(rawURL string, strip bool) (*Value, error) {
	data, err := fetchURLBytes(rawURL)
	if err != nil {
		return nil, archiveDownloadError(err)
	}
	tmp, err := os.MkdirTemp("", "goonix-tar-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	dest := filepath.Join(tmp, "unpacked")
	lm, err := unpackTarball(data, dest)
	if err != nil {
		return nil, fmt.Errorf("Failed to open archive (Unrecognized archive format)")
	}
	if strip {
		dest = stripSingleDir(dest)
	}
	meta := treeMeta{}
	if lm > 0 {
		meta.hasLast = true
		meta.lastModified = lm
	}
	return ev.treeFromDir(parser.Pos{}, dest, "source", meta, false)
}

func stripSingleDir(dir string) string {
	ents, err := os.ReadDir(dir)
	if err != nil || len(ents) != 1 || !ents[0].IsDir() {
		return dir
	}
	return filepath.Join(dir, ents[0].Name())
}

func (ev *Evaluator) fetchTreeGitHub(owner, repo, rev string) (*Value, error) {
	dir, cleanup, sha, lm, err := fetchGitHubTree(owner, repo, rev)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	meta := treeMeta{rev: sha}
	if lm > 0 {
		meta.hasLast = true
		meta.lastModified = lm
	}
	return ev.treeFromDir(parser.Pos{}, dir, "source", meta, false)
}

func fetchGitHubTree(owner, repo, rev string) (string, func(), string, int64, error) {
	sha := rev
	if len(sha) != 40 {
		body, err := downloadChecked("https://api.github.com/repos/" + owner + "/" + repo + "/commits/" + refOrHead(rev))
		if err != nil {
			return "", nil, "", 0, err
		}
		var parsed struct {
			SHA string `json:"sha"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil || len(parsed.SHA) != 40 {
			return "", nil, "", 0, fmt.Errorf("unable to parse GitHub commit for %s/%s", owner, repo)
		}
		sha = parsed.SHA
	}
	data, err := fetchURLBytes("https://codeload.github.com/" + owner + "/" + repo + "/tar.gz/" + sha)
	if err != nil {
		return "", nil, "", 0, err
	}
	tmp, err := os.MkdirTemp("", "goonix-gh-")
	if err != nil {
		return "", nil, "", 0, err
	}
	dest := filepath.Join(tmp, "unpacked")
	lm, err := unpackTarball(data, dest)
	if err != nil {
		os.RemoveAll(tmp)
		return "", nil, "", 0, fmt.Errorf("Failed to open archive (Unrecognized archive format)")
	}
	return stripSingleDir(dest), func() { os.RemoveAll(tmp) }, sha, lm, nil
}

func (ev *Evaluator) fetchTreeGitLab(owner, repo, rev string) (*Value, error) {
	project := url.PathEscape(owner + "/" + repo)
	body, err := downloadChecked("https://gitlab.com/api/v4/projects/" + project + "/repository/commits?ref_name=" + url.QueryEscape(refOrHead(rev)))
	if err != nil {
		return nil, err
	}
	var commits []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &commits); err != nil || len(commits) == 0 || commits[0].ID == "" {
		return nil, fmt.Errorf("No commits returned by GitLab API -- does the git ref really exist?")
	}
	archive := "https://gitlab.com/api/v4/projects/" + project + "/repository/archive.tar.gz?sha=" + commits[0].ID
	return ev.fetchTreeTarball(archive, true)
}

func (ev *Evaluator) fetchTreeSourceHut(owner, repo, rev string) (*Value, error) {
	ref := refOrHead(rev)
	base := "https://git.sr.ht/" + owner + "/" + repo
	if _, err := downloadChecked(base + "/" + ref); err != nil {
		return nil, err
	}
	return ev.fetchTreeTarball(base+"/archive/"+ref+".tar.gz", true)
}

func refOrHead(rev string) string {
	if rev == "" {
		return "HEAD"
	}
	return rev
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
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.Symlink(target, dest)
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
