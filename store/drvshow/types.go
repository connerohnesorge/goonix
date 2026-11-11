// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package drvshow

// Document matches `nix derivation show` top-level JSON (version 4).
type Document struct {
	Derivations map[string]Entry `json:"derivations"`
	Version     int              `json:"version"`
}

// Entry is one derivation in the show output.
type Entry struct {
	Args    []string          `json:"args"`
	Builder string            `json:"builder"`
	Env     map[string]string `json:"env"`
	Inputs  Inputs            `json:"inputs"`
	Name    string            `json:"name"`
	Outputs map[string]Output `json:"outputs"`
	System  string            `json:"system"`
	Version int               `json:"version"`
}

// Inputs holds inputDrvs and inputSrcs in nix show form.
type Inputs struct {
	Drvs map[string]InputDrv `json:"drvs"`
	Srcs []string            `json:"srcs"`
}

// InputDrv is one input derivation edge.
type InputDrv struct {
	DynamicOutputs map[string]any `json:"dynamicOutputs"`
	Outputs        []string       `json:"outputs"`
}

// Output is one derivation output path fingerprint.
type Output struct {
	Path     string `json:"path"`
	HashAlgo string `json:"hashAlgo,omitempty"`
	Hash     string `json:"hash,omitempty"`
}
