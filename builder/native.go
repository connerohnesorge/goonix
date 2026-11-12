// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
)

// A native build runs the builder directly on the host, with no sandbox, the
// way NovaNix builds on Windows. Derivation text keeps the canonical
// /nix/store spelling; at the spawn boundary every "/nix/store/" becomes the
// physical store directory followed by '/', and the output variables are the
// physical output paths. NovaNix's builds embed exactly these spellings in
// their outputs, so goonix must use them to produce the same bytes.

// nativeSourceDateEpoch is the SOURCE_DATE_EPOCH NovaNix gives every build:
// 1980-01-01, the earliest time a ZIP or PE timestamp can hold.
const nativeSourceDateEpoch = "315532800"

// nativeSpawn is everything needed to start a native builder.
type nativeSpawn struct {
	Builder  string
	Args     []string
	Env      []string
	BuildDir string
	// Files maps a file name in BuildDir to the contents to write there
	// (passAsFile and structured attrs).
	Files map[string]string
}

// nativeRewriter maps canonical store text to the physical spelling.
type nativeRewriter struct {
	storeDir string            // physical store directory, e.g. C:\nix\store
	physOuts map[string]string // output name -> physical path
}

func (r nativeRewriter) onStore(s string) string {
	if r.storeDir == store.DefaultStoreDir {
		return s
	}
	return strings.ReplaceAll(s, store.DefaultStoreDir+"/", r.storeDir+"/")
}

func (r nativeRewriter) rewrite(s string) string {
	s = r.onStore(s)
	names := make([]string, 0, len(r.physOuts))
	for name := range r.physOuts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		s = strings.ReplaceAll(s, store.OutputPlaceholder(name), r.physOuts[name])
	}
	return s
}

// envSet is an environment in which the first value set for a name wins.
// On Windows names compare case-insensitively and keep the first spelling.
type envSet struct {
	foldCase bool
	keys     []string
	vals     map[string]string
	spelling map[string]string
}

func newEnvSet(foldCase bool) *envSet {
	return &envSet{foldCase: foldCase, vals: map[string]string{}, spelling: map[string]string{}}
}

func (e *envSet) key(name string) string {
	if e.foldCase {
		return strings.ToUpper(name)
	}
	return name
}

// add sets name unless an earlier entry already did.
func (e *envSet) add(name, val string) {
	k := e.key(name)
	if _, ok := e.vals[k]; ok {
		return
	}
	e.keys = append(e.keys, k)
	e.vals[k] = val
	e.spelling[k] = name
}

// list returns NAME=value entries sorted by name, as NovaNix passes them.
func (e *envSet) list() []string {
	names := make([]string, 0, len(e.keys))
	for _, k := range e.keys {
		names = append(names, e.spelling[k])
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, n := range names {
		out = append(out, n+"="+e.vals[e.key(n)])
	}
	return out
}

// nativeBuildPath is the PATH a native builder starts with: the builder's own
// directory and the usr\bin next to its parent, so an MSYS2 bash finds its
// tools and DLLs, then the Windows system directories.
func nativeBuildPath(builder string) string {
	sys := []string{`C:\Windows\System32`, `C:\Windows`}
	dir := takeDirectory(builder)
	if dir == "" || dir == "." {
		return strings.Join(sys, ";")
	}
	parent := takeDirectory(dir)
	return strings.Join(append([]string{dir, parent + `\usr\bin`}, sys...), ";")
}

// takeDirectory drops the last component of p, splitting on '/' or '\' and
// keeping the separators before it as written (Haskell's takeDirectory on
// Windows, which NovaNix's PATH comes from).
func takeDirectory(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return "."
	}
	dir := strings.TrimRight(p[:i], `/\`)
	if dir == "" || (len(dir) == 2 && dir[1] == ':') {
		return p[:i+1]
	}
	return dir
}

// scrubAmbient keeps the few host variables a Windows process needs to start
// and drops the rest, so nothing from the user's environment reaches the build.
func scrubAmbient(ambient []string) [][2]string {
	var out [][2]string
	var systemRoot string
	haveRoot := false
	for _, kv := range ambient {
		name, val, ok := strings.Cut(kv, "=")
		if !ok || name == "" {
			continue
		}
		switch strings.ToUpper(name) {
		case "SYSTEMROOT":
			systemRoot, haveRoot = val, true
			out = append(out, [2]string{name, val})
		case "SYSTEMDRIVE", "WINDIR":
			out = append(out, [2]string{name, val})
		}
	}
	if haveRoot {
		out = append(out, [2]string{"COMSPEC", systemRoot + `\System32\cmd.exe`})
	}
	out = append(out, [2]string{"PATHEXT", ".COM;.EXE;.BAT;.CMD"})
	return out
}

// nativeBuildDir is NovaNix's build directory: the temp directory, then the
// hash part of the first output path by output name.
func nativeBuildDir(tempDir string, outPaths map[string]string) string {
	names := make([]string, 0, len(outPaths))
	for name := range outPaths {
		names = append(names, name)
	}
	sort.Strings(names)
	base := filepath.Base(outPaths[names[0]])
	hash, _, _ := strings.Cut(base, "-")
	return winJoin(tempDir, hash)
}

// winJoin joins a Windows directory and a name with one backslash, the way
// Haskell's </> does on Windows. Native builds run only on Windows; spelling
// the join out keeps the result the same when it is computed elsewhere.
func winJoin(dir, name string) string {
	return strings.TrimRight(dir, `\/`) + `\` + name
}

// prepareNativeSpawn computes the builder, arguments, environment and build
// files of a native build.
func (b *Builder) prepareNativeSpawn(d *gonixdrv.Derivation, outPaths, physOuts map[string]string, storeDir, tempDir string, ambient []string) (*nativeSpawn, error) {
	r := nativeRewriter{storeDir: storeDir, physOuts: physOuts}
	sp := &nativeSpawn{
		Builder:  r.onStore(d.Builder),
		BuildDir: nativeBuildDir(tempDir, outPaths),
		Files:    map[string]string{},
	}
	for _, a := range d.Arguments {
		sp.Args = append(sp.Args, r.rewrite(a))
	}

	// The derivation's own variables, with the output variables first so
	// they win over a derivation attribute of the same name.
	drvEnv := newEnvSet(true)
	structured := d.Env["__json"] != ""
	if structured {
		jsonBody, shBody, err := b.structuredAttrsBodies(d, outPaths, r.rewrite)
		if err != nil {
			return nil, err
		}
		sp.Files[".attrs.json"] = jsonBody
		sp.Files[".attrs.sh"] = shBody
		drvEnv.add("NIX_ATTRS_JSON_FILE", winJoin(sp.BuildDir, ".attrs.json"))
		drvEnv.add("NIX_ATTRS_SH_FILE", winJoin(sp.BuildDir, ".attrs.sh"))
	} else {
		passAsFile := map[string]bool{}
		for _, name := range strings.Fields(d.Env["passAsFile"]) {
			passAsFile[name] = true
		}
		outNames := make([]string, 0, len(physOuts))
		for name := range physOuts {
			outNames = append(outNames, name)
		}
		sort.Strings(outNames)
		for _, name := range outNames {
			if !passAsFile[name] {
				drvEnv.add(name, physOuts[name])
			}
		}
		keys := make([]string, 0, len(d.Env))
		for k := range d.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Derivation values win over the passAsFile path variables.
		for _, k := range keys {
			if !passAsFile[k] {
				drvEnv.add(r.rewrite(k), r.rewrite(d.Env[k]))
			}
		}
		for _, k := range keys {
			if passAsFile[k] {
				file := passAsFileName(k)
				sp.Files[file] = r.rewrite(d.Env[k])
				drvEnv.add(r.rewrite(k)+"Path", winJoin(sp.BuildDir, file))
			}
		}
	}

	env := newEnvSet(true)
	if isFixedOutput(d) {
		for _, name := range strings.Fields(d.Env["impureEnvVars"]) {
			v, _ := lookupEnvIn(ambient, name)
			env.add(name, v)
		}
	}
	for _, k := range drvEnv.keys {
		env.add(drvEnv.spelling[k], drvEnv.vals[k])
	}
	for _, name := range []string{"NIX_BUILD_TOP", "TMPDIR", "TEMPDIR", "TMP", "TEMP", "USERPROFILE"} {
		env.add(name, sp.BuildDir)
	}
	env.add("NIX_STORE", storeDir)
	env.add("PATH", nativeBuildPath(sp.Builder))
	env.add("SOURCE_DATE_EPOCH", nativeSourceDateEpoch)
	for _, kv := range scrubAmbient(ambient) {
		env.add(kv[0], kv[1])
	}
	sp.Env = env.list()
	return sp, nil
}

func lookupEnvIn(env []string, name string) (string, bool) {
	for _, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && k == name {
			return v, true
		}
	}
	return "", false
}

// nativeStoreDir is the physical store directory of b's store, absolute and
// with an upper-case drive letter, as NovaNix canonicalises it.
func (b *Builder) nativeStoreDir() (string, error) {
	dir, err := filepath.Abs(filepath.Join(b.Store.Root, "store"))
	if err != nil {
		return "", err
	}
	if v := filepath.VolumeName(dir); len(v) == 2 && v[1] == ':' {
		dir = strings.ToUpper(v) + dir[2:]
	}
	return dir, nil
}

// runNativeBuilder builds d by running its builder on the host, writing
// straight into the physical output paths.
func (b *Builder) runNativeBuilder(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string) error {
	if err := checkNativeSystem(drvPath, d.Platform); err != nil {
		return err
	}
	storeDir, err := b.nativeStoreDir()
	if err != nil {
		return err
	}
	physOuts := make(map[string]string, len(outPaths))
	for name, logical := range outPaths {
		physOuts[name] = local.PhysicalPath(b.Store.Root, logical)
		if err := local.RemoveStoreTree(physOuts[name]); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return err
	}
	sp, err := b.prepareNativeSpawn(d, outPaths, physOuts, storeDir, os.TempDir(), os.Environ())
	if err != nil {
		return err
	}
	if err := os.RemoveAll(sp.BuildDir); err != nil {
		return err
	}
	if err := os.MkdirAll(sp.BuildDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(sp.BuildDir)
	for name, body := range sp.Files {
		if err := os.WriteFile(filepath.Join(sp.BuildDir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	if err := spawnNative(sp); err != nil {
		removePhysOutputs(physOuts)
		return err
	}
	var missing []string
	for name, p := range physOuts {
		if _, err := os.Lstat(p); err != nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		removePhysOutputs(physOuts)
		return fmt.Errorf("builder for '%s' failed to produce output path for output '%s'", drvPath, strings.Join(missing, "', '"))
	}
	for _, p := range physOuts {
		if err := local.MakeReadOnly(p); err != nil {
			return err
		}
	}
	return b.registerRunOutputs(drvPath, d, outPaths, physOuts)
}
