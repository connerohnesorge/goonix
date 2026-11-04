// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// SkipIfUnavailable skips when nix-instantiate is missing.
func SkipIfUnavailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("nix-instantiate"); err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
}

// HermeticEnv is a minimal environment without NIX_PATH or channel config.
func (l Layout) HermeticEnv() []string {
	keep := map[string]bool{
		"HOME": true, "PATH": true, "USER": true, "LOGNAME": true,
		"TMPDIR": true, "TMP": true, "TEMP": true, "LANG": true, "LC_ALL": true,
	}
	var out []string
	for _, kv := range os.Environ() {
		key, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		switch key {
		case "NIX_PATH", "NIX_CONFIG", "NIX_USER_CONF_FILES", "NIX_REMOTE", "NIX_SSL_CERT_FILE":
			continue
		}
		if strings.HasPrefix(key, "NIX_") && key != "NIX_STORE" {
			continue
		}
		if keep[key] || strings.HasPrefix(key, "XDG_") {
			out = append(out, kv)
		}
	}
	if os.Getenv("HOME") != "" {
		found := false
		for _, kv := range out {
			if strings.HasPrefix(kv, "HOME=") {
				found = true
				break
			}
		}
		if !found {
			out = append(out, "HOME="+os.Getenv("HOME"))
		}
	}
	if os.Getenv("PATH") != "" {
		found := false
		for _, kv := range out {
			if strings.HasPrefix(kv, "PATH=") {
				found = true
				break
			}
		}
		if !found {
			out = append(out, "PATH="+os.Getenv("PATH"))
		}
	}
	return out
}

// InstantiateArgs are flags shared by every hermetic oracle invocation.
func (l Layout) InstantiateArgs() []string {
	return []string{
		"--option", "substituters", "",
		"--option", "restrict-eval", "false",
		"-I", "nixpkgs=" + l.Nixpkgs,
		"-I", "nix=" + l.Nix,
	}
}

func (l Layout) evalRaw(t *testing.T, expr string) (string, error) {
	t.Helper()
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--json", "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("nix-instantiate: %w\n%s", err, ee.Stderr)
		}
		return "", fmt.Errorf("nix-instantiate: %w", err)
	}
	return decodeNixInstantiateJSON(out)
}

func decodeNixInstantiateJSON(out []byte) (string, error) {
	var s string
	if err := json.Unmarshal(out, &s); err == nil {
		return strings.TrimSpace(s), nil
	}
	return strings.TrimSpace(string(out)), nil
}

func decodeNixInstantiateJSONPreserve(out []byte) (string, error) {
	var s string
	if err := json.Unmarshal(out, &s); err == nil {
		return s, nil
	}
	return string(out), nil
}

func (l Layout) evalFileJSON(t *testing.T, nixFile string) (string, error) {
	t.Helper()
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--json", nixFile)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("nix-instantiate %s: %w\n%s", nixFile, err, ee.Stderr)
		}
		return "", fmt.Errorf("nix-instantiate %s: %w", nixFile, err)
	}
	return decodeNixInstantiateJSON(out)
}

// EvalRaw evaluates an expression with the hermetic oracle flags.
func (l Layout) EvalRaw(t *testing.T, expr string) string {
	t.Helper()
	s, err := l.evalRaw(t, expr)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// EvalExprAllowError evaluates expr and returns an error if nix-instantiate fails.
func (l Layout) EvalExprAllowError(t *testing.T, expr string) (string, error) {
	t.Helper()
	return l.evalRaw(t, expr)
}

// EvalFileRaw evaluates a .nix file to a string (drvPath, etc.) via --json.
func (l Layout) EvalFileRaw(t *testing.T, nixFile string) string {
	t.Helper()
	s, err := l.evalFileJSON(t, nixFile)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// EvalFileRawBytes evaluates a .nix file and returns the JSON string result without trimming.
func (l Layout) EvalFileRawBytes(t *testing.T, nixFile string) string {
	t.Helper()
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--json", nixFile)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix-instantiate %s: %v\n%s", nixFile, err, out)
	}
	s, err := decodeNixInstantiateJSONPreserve(out)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Instantiate materialises a derivation and returns its .drv store path (fresh instantiate).
func (l Layout) Instantiate(t *testing.T, nixFile string) string {
	t.Helper()
	args := append(append([]string{}, l.InstantiateArgs()...), nixFile)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix-instantiate %s: %v\n%s", nixFile, err, out)
	}
	return strings.Trim(strings.TrimSpace(string(out)), `"`)
}

// EvalStrictDrvPath evaluates (builtins.derivationStrict attrs).drvPath hermetically.
func (l Layout) EvalStrictDrvPath(t *testing.T, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(builtins.derivationStrict %s).drvPath`, strictArg)
	return l.EvalRaw(t, pathExpr)
}

// EvalStdinRaw evaluates Nix source from stdin.
func (l Layout) EvalStdinRaw(t *testing.T, src string) string {
	t.Helper()
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--json", "-")
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	cmd.Stdin = strings.NewReader(src)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("nix-instantiate: %v\n%s", err, out)
	}
	s, err := decodeNixInstantiateJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// EvalStdinRawBytes is like EvalStdinRaw.
func (l Layout) EvalStdinRawBytes(t *testing.T, src string) string {
	return l.EvalStdinRaw(t, src)
}
