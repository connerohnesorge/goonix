// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

import "encoding/json"

type entryWire struct {
	Args    []string          `json:"args"`
	Builder string            `json:"builder"`
	Env     map[string]string `json:"env"`
	Inputs  *Inputs           `json:"inputs"`
	InputDrvs map[string]InputDrv `json:"inputDrvs"`
	InputSrcs []string          `json:"inputSrcs"`
	Name    string            `json:"name"`
	Outputs map[string]Output `json:"outputs"`
	System  string            `json:"system"`
	Version int               `json:"version"`
}

// UnmarshalJSON accepts Nix 2.24.x (top-level inputDrvs/inputSrcs) and 2.26+ (inputs.drvs/srcs).
func (e *Entry) UnmarshalJSON(data []byte) error {
	var w entryWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	e.Args = w.Args
	e.Builder = w.Builder
	e.Env = w.Env
	e.Name = w.Name
	e.Outputs = w.Outputs
	e.System = w.System
	e.Version = w.Version
	if w.Inputs != nil {
		e.Inputs = *w.Inputs
	} else {
		e.Inputs = Inputs{Drvs: w.InputDrvs, Srcs: w.InputSrcs}
	}
	if e.Inputs.Drvs == nil {
		e.Inputs.Drvs = map[string]InputDrv{}
	}
	if e.Inputs.Srcs == nil {
		e.Inputs.Srcs = []string{}
	}
	return nil
}
