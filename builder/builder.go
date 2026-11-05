// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
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
	order, err := TopoSort(rootDrv, b.Load)
	if err != nil {
		return err
	}
	replacements, err := b.computeReplacements(order)
	if err != nil {
		return err
	}
	for _, drvPath := range order {
		if err := b.buildOne(drvPath, replacements); err != nil {
			return fmt.Errorf("build %s: %w", drvPath, err)
		}
	}
	return nil
}

func (b *Builder) computeReplacements(order []string) (map[string]string, error) {
	repl := make(map[string]string)
	for _, drvPath := range order {
		d, err := b.Load(drvPath)
		if err != nil {
			return nil, err
		}
		r, err := d.CalculateDrvReplacement(repl)
		if err != nil {
			return nil, err
		}
		repl[drvPath] = r
	}
	return repl, nil
}

func (b *Builder) buildOne(drvPath string, replacements map[string]string) error {
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
		p = cfg.RewriteDefaultStoreDir(p)
		outPaths[name] = p
	}
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
	if isFixedOutput(d) {
		return b.buildFixedOutput(drvPath, d, outPaths)
	}
	return b.runBuilder(drvPath, d, outPaths)
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
			os.RemoveAll(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "nar", "recursive":
		if err := local.VerifyRecursiveOutput(phys, want); err != nil {
			os.RemoveAll(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "text":
		if err := verifyTextOutput(outPath, phys, want, algo); err != nil {
			os.RemoveAll(phys)
			return fixedOutputHashError(drvPath, algo, want, err)
		}
	case "git":
		if err := local.VerifyGitOutput(phys, want, algo); err != nil {
			os.RemoveAll(phys)
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
	rawURL := d.Env["url"]
	if rawURL == "" {
		return fmt.Errorf("fixed-output fetchurl missing url")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	var r io.ReadCloser
	switch u.Scheme {
	case "file":
		f, err := os.Open(u.Path)
		if err != nil {
			return err
		}
		r = f
	default:
		resp, err := http.Get(rawURL)
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return fmt.Errorf("fetchurl HTTP %s", resp.Status)
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
			return copyErr
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
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return store.CopyPathToStore(dest, phys)
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

	cmd := exec.Command(builder, args...)
	cmd.Dir = tmpDir
	cmd.Env = b.buildEnvLogical(d, outPaths, tmpDir)
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
		cmd.Env = b.buildEnvPhys(d, outPaths, physOuts, tmpDir)
		if err := cmd.Run(); err != nil {
			removePhysOutputs(physOuts)
			return err
		}
	}
	for outName, logicalOut := range outPaths {
		physOut := physOuts[outName]
		refs, err := b.Store.ScanPathReferences(logicalOut)
		if err != nil {
			return err
		}
		if err := b.Store.RecordDerivationOutput(drvPath, outName, logicalOut); err != nil {
			return err
		}
		if err := b.registerBuilt(logicalOut, physOut, refs); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) buildEnvLogical(d *gonixdrv.Derivation, outPaths map[string]string, tmpDir string) []string {
	cfg := b.Store.Config()
	storeDir := cfg.LogicalStoreDir()
	keys := make([]string, 0, len(d.Env)+8)
	for k := range d.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys)+12)
	for _, k := range keys {
		env = append(env, k+"="+b.rewriteValueLogical(d.Env[k], d, outPaths))
	}
	for name, logical := range outPaths {
		env = append(env, name+"="+logical)
	}
	cores := d.Env["NIX_BUILD_CORES"]
	if cores == "" {
		cores = "1"
	}
	env = append(env,
		"NIX_BUILD_TOP="+tmpDir,
		"TMPDIR="+tmpDir,
		"TEMP="+tmpDir,
		"TMP="+tmpDir,
		"TEMPDIR="+tmpDir,
		"NIX_BUILD_CORES="+cores,
		"NIX_STORE="+storeDir,
		"HOME=/homeless-shelter",
		"PATH=/path-not-set",
	)
	if fd := os.Getenv("NIX_LOG_FD"); fd != "" {
		env = append(env, "NIX_LOG_FD="+fd)
	} else {
		env = append(env, "NIX_LOG_FD="+strconv.Itoa(2))
	}
	return env
}

func (b *Builder) buildEnvPhys(d *gonixdrv.Derivation, outPaths, physOuts map[string]string, tmpDir string) []string {
	env := b.buildEnvLogical(d, outPaths, tmpDir)
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
	return env
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
		_ = os.RemoveAll(p)
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
