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
	"fmt"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/connerohnesorge/goonix/store"
)

func computeDerivationPaths(ev *Evaluator, cfg store.Config, drv *DerivationValue) (drvPath string, outPaths map[string]string, err error) {
	d, err := derivationValueToGoNix(drv)
	if err != nil {
		return "", nil, err
	}
	cache := ev.moduloCache
	if cache == nil {
		cache = store.NewModuloCache()
		ev.moduloCache = cache
	}
	if isFixedOutputDerivation(d) {
		outPaths, err = d.CalculateOutputPaths(nil)
	} else {
		outPaths, err = store.OutputPathsModulo(cfg, d, cache)
	}
	if err != nil {
		return "", nil, err
	}
	for name, path := range outPaths {
		if o, ok := d.Outputs[name]; ok {
			o.Path = path
		}
		d.Env[name] = path
	}
	drvPath, err = d.DrvPath()
	if err != nil {
		return "", nil, err
	}
	drvPath = cfg.RewriteDefaultStoreDir(drvPath)
	for k, v := range outPaths {
		outPaths[k] = cfg.RewriteDefaultStoreDir(v)
	}
	cache.Register(drvPath, d)
	return drvPath, outPaths, nil
}

func (ev *Evaluator) rememberStrictDerivation(drv *DerivationValue) {
	if ev == nil || drv == nil || drv.DrvPath == "" {
		return
	}
	if ev.drvStrictByPath == nil {
		ev.drvStrictByPath = make(map[string]*DerivationValue)
	}
	ev.drvStrictByPath[drv.DrvPath] = drv
	if ev.moduloCache == nil {
		ev.moduloCache = store.NewModuloCache()
	}
	gd, err := derivationValueToGoNix(drv)
	if err != nil {
		return
	}
	if drv.OutPaths != nil {
		for name, path := range drv.OutPaths {
			if o, ok := gd.Outputs[name]; ok {
				o.Path = path
			}
			gd.Env[name] = path
		}
	}
	ev.moduloCache.Register(drv.DrvPath, gd)
}

func derivationValueToGoNix(drv *DerivationValue) (*gonixdrv.Derivation, error) {
	d := &gonixdrv.Derivation{
		Outputs:          map[string]*gonixdrv.Output{},
		InputSources:     append([]string{}, drv.InputSources...),
		InputDerivations: map[string][]string{},
		Platform:         drv.System,
		Builder:          drv.Builder,
		Arguments:        append([]string{}, drv.Args...),
		Env:              map[string]string{},
	}
	for path, outs := range drv.InputDerivations {
		d.InputDerivations[path] = append([]string(nil), outs...)
	}
	for k, v := range drv.Env {
		d.Env[k] = v
	}

	outputs := drv.Outputs
	if len(outputs) == 0 {
		outputs = map[string]struct{}{"out": {}}
	}
	hashAlgo := drv.Env["outputHashAlgo"]
	hash := drv.Env["outputHash"]
	hashMode := drv.Env["outputHashMode"]
	_ = hashMode
	var parsedFixed *nixhash.Hash
	if hash != "" {
		if hw, err := store.ParseHash(hash); err == nil {
			parsedFixed = &hw.Hash
			if hashAlgo == "" {
				hashAlgo = string(hw.Hash.Algo())
			}
		}
	}
	for out := range outputs {
		o := &gonixdrv.Output{Path: ""}
		if parsedFixed != nil && out == "out" {
			if hashAlgo == "" {
				hashAlgo = "sha256"
			}
			o.HashAlgorithm = hashAlgo
			o.Hash = parsedFixed.Format(nixhash.Base16, false)
		}
		d.Outputs[out] = o
	}
	for out := range outputs {
		if _, ok := d.Env[out]; !ok {
			d.Env[out] = ""
		}
	}
	return d, nil
}

func isFixedOutputDerivation(d *gonixdrv.Derivation) bool {
	if len(d.Outputs) == 0 {
		return false
	}
	for _, o := range d.Outputs {
		if o.HashAlgorithm == "" || o.Hash == "" {
			return false
		}
	}
	return true
}

// WriteDerivation serialises a derivation value to a .drv file at its drvPath.
func WriteDerivation(cfg store.Config, drv *DerivationValue) error {
	if drv.DrvPath == "" {
		var err error
		drv.DrvPath, drv.OutPaths, err = computeDerivationPaths(nil, cfg, drv)
		if err != nil {
			return err
		}
	}
	d, err := derivationValueToGoNix(drv)
	if err != nil {
		return err
	}
	if drv.OutPaths != nil {
		for name, path := range drv.OutPaths {
			if o, ok := d.Outputs[name]; ok {
				o.Path = path
			}
			d.Env[name] = path
		}
	}
	content := &stringsBuilder{}
	if err := d.WriteDerivation(content); err != nil {
		return err
	}
	return store.WriteTextToStore(drv.DrvPath, content.String())
}

// SerializeDerivation returns the ATerm bytes for d (same as a .drv file body).
func SerializeDerivation(cfg store.Config, drv *DerivationValue) ([]byte, error) {
	if drv.DrvPath == "" {
		var err error
		drv.DrvPath, drv.OutPaths, err = computeDerivationPaths(nil, cfg, drv)
		if err != nil {
			return nil, err
		}
	}
	d, err := derivationValueToGoNix(drv)
	if err != nil {
		return nil, err
	}
	if drv.OutPaths != nil {
		for name, path := range drv.OutPaths {
			if o, ok := d.Outputs[name]; ok {
				o.Path = path
			}
			d.Env[name] = path
		}
	}
	content := &stringsBuilder{}
	if err := d.WriteDerivation(content); err != nil {
		return nil, err
	}
	return []byte(content.String()), nil
}

type stringsBuilder struct {
	b []byte
}

func (s *stringsBuilder) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}

func (s *stringsBuilder) String() string {
	return string(s.b)
}

// AttrsToDerivation converts a forced attribute set into DerivationValue.
func (ev *Evaluator) AttrsToDerivation(attrs *Value) (*DerivationValue, error) {
	return ev.attrsToDerivation(attrs)
}

// InstantiateDerivation evaluates attrs and writes the resulting .drv to the store.
func (ev *Evaluator) InstantiateDerivation(attrs *Value) (string, error) {
	drv, err := ev.attrsToDerivation(attrs)
	if err != nil {
		return "", err
	}
	cfg := store.Config{StoreDir: ev.Effects.StoreDir()}
	if err := WriteDerivation(cfg, drv); err != nil {
		return "", err
	}
	return drv.DrvPath, nil
}

// ForceDerivationAttrs ensures attrs describe a derivation and returns drvPath.
func ForceDerivationAttrs(ev *Evaluator, v *Value) (string, error) {
	fv, err := ev.ForceValue(v)
	if err != nil {
		return "", err
	}
	if fv.Kind != VKAttrs {
		return "", fmt.Errorf("not an attribute set")
	}
	typeSym := ev.Intern.Intern("type")
	b, ok := fv.Attrs.Binds[typeSym]
	if !ok {
		return "", fmt.Errorf("missing type attribute")
	}
	var tv *Value
	if b.Thunk != nil {
		tv, err = ev.forceThunk(b.Thunk)
	} else {
		tv = b.Value
	}
	if err != nil {
		return "", err
	}
	ts, err := ev.forceString(tv)
	if err != nil {
		return "", err
	}
	if ts != "derivation" {
		return "", fmt.Errorf("not a derivation")
	}
	drvSym := ev.Intern.Intern("drvPath")
	db, ok := fv.Attrs.Binds[drvSym]
	if !ok {
		return "", fmt.Errorf("missing drvPath")
	}
	var dv *Value
	if db.Thunk != nil {
		dv, err = ev.forceThunk(db.Thunk)
	} else {
		dv = db.Value
	}
	if err != nil {
		return "", err
	}
	return ev.forceString(dv)
}
