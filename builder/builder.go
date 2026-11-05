// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/store"
	"github.com/connerohnesorge/goonix/store/local"
	"github.com/connerohnesorge/goonix/substituter"
)

// Loader loads derivations by store path.
type Loader func(drvPath string) (*gonixdrv.Derivation, error)

// Substituter tries binary-cache substitution before building.
type Substituter interface {
	TrySubstitute(st *local.Store, storePath string) (bool, error)
}

// Config controls builder behaviour.
type Config struct {
	// Sandbox enables a minimal build sandbox (default off; builds run unsandboxed).
	Sandbox bool
	// Substituter is optional; when set, output paths are fetched before building.
	Substituter Substituter
}

// Builder executes derivations into a local store.
type Builder struct {
	Store  *local.Store
	Config Config
	Load   Loader
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
	outPaths, err := d.CalculateOutputPaths(replacements)
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
	if err := os.MkdirAll(filepath.Dir(phys), 0o755); err != nil {
		return err
	}
	// FOD fetchurl-style derivations must be populated out-of-band (network) by caller;
	// here we only verify if the path already exists on disk.
	if _, err := os.Stat(phys); err != nil {
		return fmt.Errorf("fixed-output %s not present (network fetch not implemented in builder): %w", outPath, err)
	}
	want := d.Outputs["out"].Hash
	switch mode {
	case "flat", "":
		if err := local.VerifyFlatOutput(phys, want); err != nil {
			return err
		}
	case "nar", "recursive":
		if err := local.VerifyRecursiveOutput(phys, want); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported outputHashMode %q", mode)
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
	if b.Config.Sandbox {
		return fmt.Errorf("sandbox mode not implemented")
	}
	buildTop := filepath.Join(b.Store.Root, "build-top")
	if err := os.MkdirAll(buildTop, 0o777); err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp(buildTop, "goonix-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	physOuts := make(map[string]string, len(outPaths))
	for name, logical := range outPaths {
		phys := local.PhysicalPath(b.Store.Root, logical)
		physOuts[name] = phys
	}
	if b.outputsNeedPrecreatedDirs(d, outPaths) {
		for _, phys := range physOuts {
			if err := os.MkdirAll(phys, 0o755); err != nil {
				return err
			}
		}
	}
	builder := d.Builder
	args := append([]string{}, d.Arguments...)
	for i, a := range args {
		args[i] = b.rewriteValue(a, d, outPaths, physOuts)
	}

	cmd := exec.Command(builder, args...)
	cmd.Dir = tmpDir
	cmd.Env = b.buildEnvWithPhys(d, outPaths, physOuts, tmpDir, buildTop)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
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

func (b *Builder) buildEnvWithPhys(d *gonixdrv.Derivation, outPaths, physOuts map[string]string, tmpDir, buildTop string) []string {
	cfg := b.Store.Config()
	storeDir := cfg.LogicalStoreDir()
	keys := make([]string, 0, len(d.Env)+8)
	for k := range d.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys)+8)
	for _, k := range keys {
		env = append(env, k+"="+b.rewriteValue(d.Env[k], d, outPaths, physOuts))
	}
	for name, phys := range physOuts {
		env = append(env, strings.ToUpper(name)+"="+phys)
		env = append(env, name+"="+phys)
	}
	env = append(env,
		"NIX_BUILD_TOP="+buildTop,
		"TMPDIR="+tmpDir,
		"NIX_STORE="+storeDir,
		"HOME=/homeless-shelter",
		"PATH=/path-not-set",
	)
	return env
}

func (b *Builder) rewriteValue(val string, d *gonixdrv.Derivation, outPaths, physOuts map[string]string) string {
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
	}
	return cfg.RewriteDefaultStoreDir(out)
}

func (b *Builder) outputsNeedPrecreatedDirs(d *gonixdrv.Derivation, outPaths map[string]string) bool {
	check := func(s string) bool {
		for name := range outPaths {
			if strings.Contains(s, "$"+name+"/") || strings.Contains(s, "${"+name+"}/") {
				return true
			}
			if logical, ok := outPaths[name]; ok && strings.Contains(s, logical+"/") {
				return true
			}
		}
		return false
	}
	for _, a := range d.Arguments {
		if check(a) {
			return true
		}
	}
	for _, v := range d.Env {
		if check(v) {
			return true
		}
	}
	return false
}

// DefaultSubstituter returns a cache.nixos.org client.
func DefaultSubstituter() Substituter {
	return substituter.NewClient(substituter.Config{})
}
