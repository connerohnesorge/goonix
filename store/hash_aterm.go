// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"io"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"
)

// inputReplacements maps input derivation store paths to modulo hex keys (Nix inputs2).
type inputReplacements map[string]string

// WriteDerivationForHash serialises d in ATerm form for hashDerivationModulo.
func WriteDerivationForHash(w io.Writer, d *gonixdrv.Derivation, maskOutputs bool, reps inputReplacements) error {
	return writeDerivationForHash(w, d, maskOutputs, reps)
}

func writeDerivationForHash(w io.Writer, d *gonixdrv.Derivation, maskOutputs bool, reps inputReplacements) error {
	outputNames := make([]string, 0, len(d.Outputs))
	for k := range d.Outputs {
		outputNames = append(outputNames, k)
	}
	sort.Strings(outputNames)

	inputDerivations := d.InputDerivations
	if len(reps) > 0 {
		inputDerivations = make(map[string][]string, len(d.InputDerivations))
		for drvPath, outputNames := range d.InputDerivations {
			replacement, ok := reps[drvPath]
			if !ok {
				return errMissingInputReplacement(drvPath)
			}
			inputDerivations[replacement] = append([]string(nil), outputNames...)
		}
	}

	inputKeys := make([]string, 0, len(inputDerivations))
	for k := range inputDerivations {
		inputKeys = append(inputKeys, k)
	}
	sort.Strings(inputKeys)

	envKeys := make([]string, 0, len(d.Env))
	for k := range d.Env {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	if _, err := io.WriteString(w, "Derive("); err != nil {
		return err
	}
	if err := writeTupleList(w, outputNames, func(name string) error {
		o := d.Outputs[name]
		path := o.Path
		if maskOutputs {
			path = ""
		}
		return writeQuotedTuple(w, name, path, o.HashAlgorithm, o.Hash)
	}); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ","); err != nil {
		return err
	}
	if err := writeInputDrvList(w, inputKeys, inputDerivations); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ","); err != nil {
		return err
	}
	srcs := append([]string(nil), d.InputSources...)
	sort.Strings(srcs)
	if err := writeStringList(w, srcs); err != nil {
		return err
	}
	if err := writeQuotedString(w, d.Platform); err != nil {
		return err
	}
	if err := writeQuotedString(w, d.Builder); err != nil {
		return err
	}
	if _, err := io.WriteString(w, ","); err != nil {
		return err
	}
	args := append([]string(nil), d.Arguments...)
	if err := writeStringList(w, args); err != nil {
		return err
	}
	if err := writeEnvList(w, envKeys, d.Env, d.Outputs, maskOutputs); err != nil {
		return err
	}
	_, err := io.WriteString(w, ")")
	return err
}

type missingInputReplacementError struct{ path string }

func errMissingInputReplacement(path string) error {
	return missingInputReplacementError{path: path}
}

func (e missingInputReplacementError) Error() string {
	return "missing input derivation replacement for " + e.path
}

func writeTupleList(w io.Writer, keys []string, fn func(string) error) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, k := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "("); err != nil {
			return err
		}
		if err := fn(k); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ")"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

func writeInputDrvList(w io.Writer, keys []string, m map[string][]string) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, k := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "("); err != nil {
			return err
		}
		if err := writeNixString(w, k); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ","); err != nil {
			return err
		}
		outs := append([]string(nil), m[k]...)
		sort.Strings(outs)
		if err := writeStringList(w, outs); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ")"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

func writeEnvList(w io.Writer, keys []string, env map[string]string, outputs map[string]*gonixdrv.Output, maskOutputs bool) error {
	if _, err := io.WriteString(w, ",["); err != nil {
		return err
	}
	for i, k := range keys {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		val := env[k]
		if maskOutputs {
			if _, isOut := outputs[k]; isOut {
				val = ""
			}
		}
		if _, err := io.WriteString(w, "("); err != nil {
			return err
		}
		if err := writeNixString(w, k); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ","); err != nil {
			return err
		}
		if err := writeNixString(w, val); err != nil {
			return err
		}
		if _, err := io.WriteString(w, ")"); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

func writeStringList(w io.Writer, elems []string) error {
	if _, err := io.WriteString(w, "["); err != nil {
		return err
	}
	for i, e := range elems {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := writeNixString(w, e); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, "]")
	return err
}

func writeQuotedTuple(w io.Writer, parts ...string) error {
	for i, p := range parts {
		if i > 0 {
			if _, err := io.WriteString(w, ","); err != nil {
				return err
			}
		}
		if err := writeNixString(w, p); err != nil {
			return err
		}
	}
	return nil
}

func writeQuotedString(w io.Writer, s string) error {
	if _, err := io.WriteString(w, ","); err != nil {
		return err
	}
	return writeNixString(w, s)
}

func writeNixString(w io.Writer, s string) error {
	if _, err := io.WriteString(w, `"`); err != nil {
		return err
	}
	if err := writeEscapedString(w, s); err != nil {
		return err
	}
	_, err := io.WriteString(w, `"`)
	return err
}

func writeEscapedString(w io.Writer, s string) error {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
	_, err := io.WriteString(w, r.Replace(s))
	return err
}
