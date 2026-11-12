// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"

	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
	"github.com/connerohnesorge/goonix/substituter"
)

// errMountFallback signals that the /nix/store bind mount could not be set up.
// runBuilder may rewrite paths only when AllowRelocated is set. A builder that
// already ran and exited nonzero is not this error.
var errMountFallback = fmt.Errorf("mount namespace unavailable")

// Loader loads derivations by store path.
type Loader func(drvPath string) (*gonixdrv.Derivation, error)

// Substituter tries binary-cache substitution before building.
type Substituter interface {
	TrySubstitute(st *local.Store, storePath string) (bool, error)
}

// Config controls builder behaviour.
type Config struct {
	// Substituter is optional; when set, output paths are fetched before building.
	Substituter Substituter
	// AllowRelocated permits physical-path rewriting when a /nix/store bind mount
	// is unavailable. Default false: builds fail unless the builder runs with logical
	// store paths under /nix/store (see execBuildCommandMount).
	AllowRelocated bool
}

// Builder executes derivations into a local store.
type Builder struct {
	Store  *local.Store
	Config Config
	Load   Loader
	// plan is the sandbox view for the build currently running.
	plan buildPlan
}

// buildPlan lists the store paths mounted into the sandbox and the outputs
// copied back to the host store after a successful builder.
type buildPlan struct {
	closure    []string
	outputs    []string
	isolateNet bool
}

// BuildRealisation builds rootDrv and everything it depends on.
func (b *Builder) BuildRealisation(rootDrv string) error {
	// Hold off garbage collection until this build is done.
	unlock, err := b.Store.LockForBuild()
	if err != nil {
		return err
	}
	defer unlock()
	if b.Load == nil {
		b.Load = func(p string) (*gonixdrv.Derivation, error) {
			f, err := os.Open(local.PhysicalPath(b.Store.Root, p))
			if err != nil {
				return nil, err
			}
			defer f.Close()
			return gonixdrv.ReadDerivation(f)
		}
	}
	// TopoSort rejects a cyclic or unloadable graph before anything runs.
	if _, err := TopoSort(rootDrv, b.Load); err != nil {
		return err
	}
	r := &realiser{b: b, built: map[string]bool{}}
	return r.realise(rootDrv, nil)
}

// realiser makes derivation outputs valid the way Nix's goals do: a wanted
// output that is already valid or can be substituted is done, and only a
// derivation that has to be built has its inputs realised. So a cached
// package is fetched with its runtime closure, and its build-time inputs
// (the compiler, the bootstrap stages) are never fetched or built.
type realiser struct {
	b     *Builder
	built map[string]bool
}

// realise makes the outputs in wanted valid (every output when wanted is
// empty), substituting or building drvPath as needed.
func (r *realiser) realise(drvPath string, wanted []string) error {
	b := r.b
	d, err := b.Load(drvPath)
	if err != nil {
		return err
	}
	outPaths, err := store.LogicalOutputPaths(d)
	if err != nil {
		return err
	}
	cfg := b.Store.Config()
	for name, p := range outPaths {
		outPaths[name] = cfg.RewriteDefaultStoreDir(p)
	}
	names := wanted
	if len(names) == 0 {
		for name := range outPaths {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	done := true
	for _, name := range names {
		p, ok := outPaths[name]
		if !ok {
			return fmt.Errorf("derivation '%s' does not have wanted outputs '%s'", drvPath, name)
		}
		valid, err := b.Store.IsValidPath(p)
		if err != nil {
			return err
		}
		if valid {
			continue
		}
		if b.trySubstitutePath(p) {
			continue
		}
		done = false
		break
	}
	if done {
		return nil
	}
	if r.built[drvPath] {
		return fmt.Errorf("build %s: outputs %v are not valid after building", drvPath, names)
	}
	r.built[drvPath] = true
	inputs := make([]string, 0, len(d.InputDerivations))
	for inp := range d.InputDerivations {
		inputs = append(inputs, inp)
	}
	sort.Strings(inputs)
	for _, inp := range inputs {
		if err := r.realise(inp, d.InputDerivations[inp]); err != nil {
			return err
		}
	}
	if err := b.buildOne(drvPath, d, outPaths); err != nil {
		return fmt.Errorf("build %s: %w", drvPath, err)
	}
	return nil
}

// buildOne builds d. Outputs that are already valid are left alone; when
// every output is valid there is nothing to do.
func (b *Builder) buildOne(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string) error {
	allValid := true
	for _, p := range outPaths {
		ok, err := b.Store.IsValidPath(p)
		if err != nil {
			return err
		}
		if !ok {
			allValid = false
			break
		}
	}
	if allValid {
		return nil
	}
	if b.trySubstituteOutputs(outPaths) {
		return nil
	}
	if d.Builder == builtinUnpack {
		return b.buildBuiltinUnpack(drvPath, d, outPaths)
	}
	if isFixedOutput(d) {
		return b.buildFixedOutput(drvPath, d, outPaths)
	}
	return b.runBuilder(drvPath, d, outPaths)
}

// trySubstitutePath substitutes one store path and its references.
func (b *Builder) trySubstitutePath(p string) bool {
	sub := b.Config.Substituter
	if sub == nil {
		return false
	}
	ok, err := sub.TrySubstitute(b.Store, p)
	return err == nil && ok
}

func (b *Builder) trySubstituteOutputs(outPaths map[string]string) bool {
	sub := b.Config.Substituter
	if sub == nil {
		return false
	}
	for _, p := range outPaths {
		ok, err := sub.TrySubstitute(b.Store, p)
		if err != nil || !ok {
			return false
		}
	}
	return true
}

func isFixedOutput(d *gonixdrv.Derivation) bool {
	if len(d.Outputs) != 1 {
		return false
	}
	o, ok := d.Outputs["out"]
	return ok && o != nil && o.HashAlgorithm != ""
}

func (b *Builder) buildFixedOutput(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string) error {
	outPath := outPaths["out"]
	mode := d.Env["outputHashMode"]
	phys := local.PhysicalPath(b.Store.Root, outPath)
	if d.Builder == "builtin:fetchurl" {
		if err := b.fetchFixedOutputURL(d, phys, mode); err != nil {
			return err
		}
	} else {
		ok, err := b.Store.IsValidPath(outPath)
		if err != nil {
			return err
		}
		if !ok {
			if err := b.runBuilder(drvPath, d, outPaths); err != nil {
				return err
			}
		}
	}
	want := d.Outputs["out"].Hash
	algo := d.Env["outputHashAlgo"]
	if algo == "" {
		algo = d.Outputs["out"].HashAlgorithm
	}
	switch mode {
	case "flat", "":
		if err := local.VerifyFlatOutput(phys, want, algo); err != nil {
			_ = local.RemoveStoreTree(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "nar", "recursive":
		if err := local.VerifyRecursiveOutput(phys, want); err != nil {
			_ = local.RemoveStoreTree(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "text":
		if err := verifyTextOutput(outPath, phys, want, algo); err != nil {
			_ = local.RemoveStoreTree(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "git":
		if err := local.VerifyGitOutput(phys, want, algo); err != nil {
			_ = local.RemoveStoreTree(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	default:
		return fmt.Errorf("invalid value '%s' for 'outputHashMode' attribute", mode)
	}
	refs, err := b.Store.ScanPathReferences(outPath)
	if err != nil {
		return err
	}
	if err := b.Store.RecordDerivationOutput(drvPath, "out", outPath); err != nil {
		return err
	}
	return b.registerBuilt(outPath, phys, refs)
}

func fixedOutputHashError(drvPath, algo, want string, err error) error {
	var mm *local.OutputHashMismatch
	if !errors.As(err, &mm) {
		return err
	}
	spec, err1 := sriFromBase16(algo, want)
	got, err2 := sriFromBase16(algo, mm.GotBase16)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("hash mismatch in fixed-output derivation '%s':\n  specified: %s\n     got:    %s", drvPath, want, mm.GotBase16)
	}
	return fmt.Errorf("hash mismatch in fixed-output derivation '%s':\n  specified: %s\n     got:    %s", drvPath, spec, got)
}

func sriFromBase16(algo, base16 string) (string, error) {
	algo = strings.TrimPrefix(algo, "text:")
	algo = strings.TrimPrefix(algo, "git:")
	if algo == "" {
		algo = "sha256"
	}
	raw, err := hex.DecodeString(base16)
	if err != nil {
		return "", err
	}
	var kind nixhash.Algorithm
	switch algo {
	case "md5":
		kind = nixhash.MD5
	case "sha1":
		kind = nixhash.SHA1
	case "sha256":
		kind = nixhash.SHA256
	case "sha512":
		kind = nixhash.SHA512
	default:
		return "", fmt.Errorf("unknown hash algorithm '%s'", algo)
	}
	h, err := nixhash.NewHash(kind, raw)
	if err != nil {
		return "", err
	}
	return h.Format(nixhash.SRI, true), nil
}

func verifyTextOutput(logical, phys, want, algo string) error {
	fi, err := os.Lstat(phys)
	if err != nil {
		return err
	}
	if !fi.Mode().IsRegular() || fi.Mode()&0o111 != 0 {
		return fmt.Errorf("output path '%s' should be a non-executable regular file since recursive hashing is not enabled (one of outputHashMode={flat,text} is true)", logical)
	}
	raw := strings.TrimPrefix(algo, "text:")
	if raw == "" {
		raw = "sha256"
	}
	return local.VerifyFlatOutput(phys, want, raw)
}

func (b *Builder) fetchFixedOutputURL(d *gonixdrv.Derivation, phys, hashMode string) error {
	urls := fetchURLs(d)
	if len(urls) == 0 {
		return fmt.Errorf("fixed-output fetchurl missing url")
	}
	// NovaNix's `urls` list: try each in order, and move on after any
	// failure, including a download whose hash is wrong. The last URL's
	// outcome is what the caller verifies and reports.
	var errs []string
	for i, rawURL := range urls {
		err := b.fetchOneURL(rawURL, phys, hashMode)
		if err == nil && i < len(urls)-1 && (hashMode == "flat" || hashMode == "") {
			err = verifyFetchedFlat(d, phys)
		}
		if err == nil {
			return nil
		}
		_ = local.RemoveStoreTree(phys)
		if len(urls) == 1 {
			return err
		}
		errs = append(errs, fmt.Sprintf("%s: %v", rawURL, err))
	}
	return fmt.Errorf("builtin:fetchurl: every URL failed:\n%s", strings.Join(errs, "\n"))
}

// fetchURLs is the derivation's `urls` (whitespace-separated, as NovaNix's
// fetchurl writes it) or, without one, its `url`.
func fetchURLs(d *gonixdrv.Derivation) []string {
	if v, ok := d.Env["urls"]; ok {
		return strings.FieldsFunc(v, func(r rune) bool {
			return r < 0x80 && (r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' || r == '\v')
		})
	}
	if v := d.Env["url"]; v != "" {
		return []string{v}
	}
	return nil
}

func verifyFetchedFlat(d *gonixdrv.Derivation, phys string) error {
	algo := d.Env["outputHashAlgo"]
	if algo == "" {
		algo = d.Outputs["out"].HashAlgorithm
	}
	return local.VerifyFlatOutput(phys, d.Outputs["out"].Hash, algo)
}

// fetchAttempts and fetchRetryBase are NovaNix's per-URL retry policy for
// transient failures: five attempts, exponential backoff from 250ms.
const (
	fetchAttempts  = 5
	fetchRetryBase = 250 * time.Millisecond
)

type transientFetchError struct{ err error }

func (e transientFetchError) Error() string { return e.err.Error() }

func (b *Builder) fetchOneURL(rawURL, phys, hashMode string) error {
	var err error
	for attempt := 0; attempt < fetchAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(fetchRetryBase << (attempt - 1))
		}
		err = b.fetchOneURLOnce(rawURL, phys, hashMode)
		var te transientFetchError
		if err == nil || !errors.As(err, &te) {
			return err
		}
		_ = local.RemoveStoreTree(phys)
	}
	return err
}

// transientStatus reports whether an HTTP status is worth retrying: server
// errors and 408/429, but not the 5xx codes that cannot change on retry.
func transientStatus(code int) bool {
	switch code {
	case 408, 429:
		return true
	case 501, 505, 511:
		return false
	}
	return code >= 500
}

func (b *Builder) fetchOneURLOnce(rawURL, phys, hashMode string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	var r io.ReadCloser
	switch u.Scheme {
	case "file":
		f, err := os.Open(fileURLPath(u))
		if err != nil {
			return err
		}
		r = f
	default:
		resp, err := http.Get(rawURL)
		if err != nil {
			return transientFetchError{err}
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err := fmt.Errorf("fetchurl HTTP %s", resp.Status)
			if transientStatus(resp.StatusCode) {
				return transientFetchError{err}
			}
			return err
		}
		r = resp.Body
	}
	defer r.Close()
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		return err
	}
	if hashMode == "flat" || hashMode == "" {
		f, err := os.OpenFile(phys, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o444)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(f, r)
		closeErr := f.Close()
		if copyErr != nil {
			return transientFetchError{copyErr}
		}
		return closeErr
	}
	tmp, err := os.MkdirTemp(filepath.Dir(phys), "fod-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	dest := filepath.Join(tmp, "tree")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dest, "downloaded"))
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return transientFetchError{err}
	}
	if err := f.Close(); err != nil {
		return err
	}
	return store.CopyPathToStore(dest, phys)
}

// fileURLPath is the local path of a file:// URL. On Windows the URL path of
// file:///C:/x is /C:/x, and the leading slash is not part of the path.
func fileURLPath(u *url.URL) string {
	p := u.Path
	if runtime.GOOS == "windows" && len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

func (b *Builder) registerBuilt(logicalPath, phys string, refs []string) error {
	narHash, narSize, err := local.NarHashAndSize(phys)
	if err != nil {
		narHash, narSize, err = local.FlatNarHashAndSize(phys)
		if err != nil {
			return err
		}
	}
	return b.Store.RegisterValidPath(logicalPath, narHash, narSize, refs)
}

func (b *Builder) runBuilder(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string) error {
	if nativeBuilds {
		return b.runNativeBuilder(drvPath, d, outPaths)
	}
	cfg := b.Store.Config()
	if cfg.LogicalStoreDir() != store.DefaultStoreDir && !b.Config.AllowRelocated {
		return fmt.Errorf("logical store directory is %q (expected %q); set AllowRelocated only for tests", cfg.LogicalStoreDir(), store.DefaultStoreDir)
	}
	tmpDir, err := os.MkdirTemp("", "goonix-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	physOuts := make(map[string]string, len(outPaths))
	for name, logical := range outPaths {
		physOuts[name] = local.PhysicalPath(b.Store.Root, logical)
	}
	b.plan = buildPlan{
		closure:    b.closurePhys(d, outPaths),
		outputs:    outputPhysList(physOuts),
		isolateNet: !isFixedOutput(d),
	}
	defer func() { b.plan = buildPlan{} }()
	builder := d.Builder
	args := append([]string{}, d.Arguments...)
	for i, a := range args {
		args[i] = b.rewriteValueLogical(a, d, outPaths)
	}

	if err := b.writeStructuredAttrsFiles(d, outPaths, tmpDir); err != nil {
		return err
	}
	cmd := exec.Command(builder, args...)
	cmd.Dir = tmpDir
	env, err := b.buildEnvLogical(d, outPaths, tmpDir)
	if err != nil {
		return err
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := b.execBuildCommand(cmd); err != nil {
		if err != errMountFallback {
			removePhysOutputs(physOuts)
			return err
		}
		if !b.Config.AllowRelocated {
			removePhysOutputs(physOuts)
			return fmt.Errorf("cannot build with logical /nix/store paths: %w (set AllowRelocated for tests only, or enable user namespaces for bind mount)", err)
		}
		for i, a := range args {
			args[i] = b.rewriteValuePhys(a, d, outPaths, physOuts)
		}
		cmd.Args = append([]string{builder}, args...)
		env, err = b.buildEnvPhys(d, outPaths, physOuts, tmpDir)
		if err != nil {
			removePhysOutputs(physOuts)
			return err
		}
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			removePhysOutputs(physOuts)
			return err
		}
	}
	return b.registerRunOutputs(drvPath, d, outPaths, physOuts)
}

// registerRunOutputs scans, checks and registers the outputs a builder wrote
// at physOuts.
func (b *Builder) registerRunOutputs(drvPath string, d *gonixdrv.Derivation, outPaths, physOuts map[string]string) error {
	also := make([]string, 0, len(outPaths))
	for _, logicalOut := range outPaths {
		also = append(also, logicalOut)
	}
	infos := make(map[string]builtOutput, len(outPaths))
	for outName, logicalOut := range outPaths {
		physOut := physOuts[outName]
		refs, err := b.Store.ScanPathReferencesAmong(logicalOut, also)
		if err != nil {
			removePhysOutputs(physOuts)
			return err
		}
		_, narSize, err := local.NarHashAndSize(physOut)
		if err != nil {
			removePhysOutputs(physOuts)
			return err
		}
		infos[outName] = builtOutput{refs: refs, narSize: narSize}
	}
	if err := b.finishStructuredOutputs(drvPath, d, outPaths, infos); err != nil {
		removePhysOutputs(physOuts)
		return err
	}
	for outName, logicalOut := range outPaths {
		physOut := physOuts[outName]
		info := infos[outName]
		if err := b.Store.RecordDerivationOutput(drvPath, outName, logicalOut); err != nil {
			return err
		}
		if err := b.registerBuilt(logicalOut, physOut, info.refs); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) buildEnvLogical(d *gonixdrv.Derivation, outPaths map[string]string, tmpDir string) ([]string, error) {
	cfg := b.Store.Config()
	storeDir := cfg.LogicalStoreDir()
	structured := d.Env["__json"] != ""
	passAsFile := map[string]struct{}{}
	if !structured {
		for _, name := range strings.Fields(d.Env["passAsFile"]) {
			if name != "" {
				passAsFile[name] = struct{}{}
			}
		}
	}
	keys := make([]string, 0, len(d.Env)+8)
	if !structured {
		for k := range d.Env {
			if _, skip := passAsFile[k]; skip {
				continue
			}
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys)+12)
	for _, k := range keys {
		env = append(env, k+"="+b.rewriteValueLogical(d.Env[k], d, outPaths))
	}
	if !structured {
		for name, logical := range outPaths {
			env = append(env, name+"="+logical)
		}
		if err := b.writePassAsFiles(passAsFile, d, tmpDir, func(v string) string {
			return b.rewriteValueLogical(v, d, outPaths)
		}); err != nil {
			return nil, err
		}
		for name := range passAsFile {
			env = append(env, name+"Path="+filepath.Join(tmpDir, passAsFileName(name)))
		}
	}
	env = append(env, b.structuredAttrsEnv(d, tmpDir)...)
	env = append(env,
		"NIX_BUILD_TOP="+tmpDir,
		"TMPDIR="+tmpDir,
		"TEMP="+tmpDir,
		"TMP="+tmpDir,
		"TEMPDIR="+tmpDir,
		"NIX_STORE="+storeDir,
		"HOME=/homeless-shelter",
		"PATH=/path-not-set",
	)
	// A derivation that sets NIX_BUILD_CORES keeps that value. Otherwise the
	// cores setting is used, and an unset setting means all CPUs. 0 is passed
	// through: GNU make treats -j0 as every CPU. TERM is always the value Nix
	// 2.24.9 writes after the derivation environment.
	env = setEnv(env, "NIX_BUILD_CORES", builderCores(d.Env["NIX_BUILD_CORES"]))
	env = setEnv(env, "TERM", "xterm-256color")
	if fd := os.Getenv("NIX_LOG_FD"); fd != "" {
		env = append(env, "NIX_LOG_FD="+fd)
	} else {
		env = append(env, "NIX_LOG_FD="+strconv.Itoa(2))
	}
	return env, nil
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + val
			return env
		}
	}
	return append(env, prefix+val)
}

func builderCores(drvValue string) string {
	if drvValue != "" {
		return drvValue
	}
	if v, ok := builderNixSetting("cores"); ok && v != "" {
		return v
	}
	n := runtime.NumCPU()
	if n < 1 {
		n = 1
	}
	return strconv.Itoa(n)
}

func builderNixSetting(key string) (string, bool) {
	if v, ok := parseNixConfText(os.Getenv("NIX_CONFIG"))[key]; ok {
		return v, true
	}
	dirs := os.Getenv("NIX_CONF_DIR")
	if dirs != "" {
		for _, dir := range strings.Split(dirs, ":") {
			if dir == "" {
				continue
			}
			if v, ok := parseNixConfFile(filepath.Join(dir, "nix.conf"))[key]; ok {
				return v, true
			}
		}
		return "", false
	}
	if v, ok := parseNixConfFile("/etc/nix/nix.conf")[key]; ok {
		return v, true
	}
	return "", false
}

func parseNixConfFile(path string) map[string]string {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseNixConfText(string(body))
}

func parseNixConfText(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func passAsFileName(name string) string {
	sum := sha256.Sum256([]byte(name))
	return ".attr-" + nixbase32.EncodeToString(sum[:])
}

func (b *Builder) writePassAsFiles(names map[string]struct{}, d *gonixdrv.Derivation, tmpDir string, rewrite func(string) string) error {
	for name := range names {
		body := rewrite(d.Env[name])
		if err := os.WriteFile(filepath.Join(tmpDir, passAsFileName(name)), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) buildEnvPhys(d *gonixdrv.Derivation, outPaths, physOuts map[string]string, tmpDir string) ([]string, error) {
	env, err := b.buildEnvLogical(d, outPaths, tmpDir)
	if err != nil {
		return nil, err
	}
	for i, e := range env {
		key, val, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if phys, ok := physOuts[key]; ok {
			env[i] = key + "=" + phys
			continue
		}
		env[i] = key + "=" + b.rewriteValuePhys(val, d, outPaths, physOuts)
	}
	if d.Env["__json"] == "" {
		names := map[string]struct{}{}
		for _, name := range strings.Fields(d.Env["passAsFile"]) {
			if name != "" {
				names[name] = struct{}{}
			}
		}
		if err := b.writePassAsFiles(names, d, tmpDir, func(v string) string {
			return b.rewriteValuePhys(v, d, outPaths, physOuts)
		}); err != nil {
			return nil, err
		}
	}
	return env, nil
}

func (b *Builder) rewriteValuePhys(val string, d *gonixdrv.Derivation, outPaths, physOuts map[string]string) string {
	cfg := b.Store.Config()
	out := val
	for name, logical := range outPaths {
		phys := physOuts[name]
		out = strings.ReplaceAll(out, store.OutputPlaceholder(name), phys)
		out = strings.ReplaceAll(out, cfg.RewriteDefaultStoreDir(store.OutputPlaceholder(name)), phys)
		out = strings.ReplaceAll(out, logical, phys)
	}
	for _, src := range d.InputSources {
		phys := local.PhysicalPath(b.Store.Root, src)
		out = strings.ReplaceAll(out, src, phys)
	}
	for inp := range d.InputDerivations {
		out = strings.ReplaceAll(out, inp, local.PhysicalPath(b.Store.Root, inp))
		if b.Load != nil {
			if dep, err := b.Load(inp); err == nil && dep != nil {
				if depOuts, err := store.LogicalOutputPaths(dep); err == nil {
					for _, logical := range depOuts {
						out = strings.ReplaceAll(out, logical, local.PhysicalPath(b.Store.Root, logical))
					}
				}
			}
		}
	}
	return cfg.RewriteDefaultStoreDir(out)
}

func (b *Builder) rewriteValueLogical(val string, d *gonixdrv.Derivation, outPaths map[string]string) string {
	cfg := b.Store.Config()
	out := val
	for name, logical := range outPaths {
		out = strings.ReplaceAll(out, store.OutputPlaceholder(name), logical)
		out = strings.ReplaceAll(out, cfg.RewriteDefaultStoreDir(store.OutputPlaceholder(name)), logical)
	}
	for _, src := range d.InputSources {
		out = strings.ReplaceAll(out, src, src)
	}
	for inp := range d.InputDerivations {
		out = strings.ReplaceAll(out, inp, inp)
	}
	return cfg.RewriteDefaultStoreDir(out)
}

func outputPhysList(physOuts map[string]string) []string {
	out := make([]string, 0, len(physOuts))
	for _, p := range physOuts {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func removePhysOutputs(physOuts map[string]string) {
	for _, p := range physOuts {
		_ = local.RemoveStoreTree(p)
	}
}

// closurePhys is the physical paths of the derivation's inputs and their references.
// Outputs of this build are not included.
func (b *Builder) closurePhys(d *gonixdrv.Derivation, outPaths map[string]string) []string {
	skip := map[string]bool{}
	for _, logical := range outPaths {
		skip[logical] = true
	}
	seen := map[string]bool{}
	var phys []string
	var walk func(logical string)
	walk = func(logical string) {
		if logical == "" || skip[logical] || seen[logical] {
			return
		}
		seen[logical] = true
		p := local.PhysicalPath(b.Store.Root, logical)
		if st, err := os.Lstat(p); err != nil || st == nil {
			return
		}
		phys = append(phys, p)
		refs, err := b.Store.QueryReferences(logical)
		if err != nil {
			return
		}
		for _, ref := range refs {
			walk(ref)
		}
	}
	for _, src := range d.InputSources {
		walk(src)
	}
	for inp := range d.InputDerivations {
		walk(inp)
		if b.Load == nil {
			continue
		}
		dep, err := b.Load(inp)
		if err != nil || dep == nil {
			continue
		}
		depOuts, err := store.LogicalOutputPaths(dep)
		if err != nil {
			continue
		}
		for _, logical := range depOuts {
			walk(logical)
		}
	}
	sort.Strings(phys)
	return phys
}

// DefaultSubstituter returns a cache.nixos.org client.
func DefaultSubstituter() Substituter {
	return substituter.NewClient(substituter.Config{})
}
