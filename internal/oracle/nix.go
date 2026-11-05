// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (l Layout) instantiateArgsWithStore(storeDir string) []string {
	args := append([]string{}, l.InstantiateArgs()...)
	if storeDir != "" {
		args = append([]string{"--store", storeDir}, args...)
	}
	return args
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

// RealiseDrvInStore ensures drvPath exists in storeDir and returns its .drv bytes.
func (l Layout) RealiseDrvInStore(t *testing.T, storeDir, drvPath string) []byte {
	t.Helper()
	if raw, err := os.ReadFile(l.storeDrvPath(storeDir, drvPath)); err == nil {
		return raw
	}
	args := []string{"--realise", drvPath}
	if storeDir != "" {
		args = append([]string{"--store", storeDir}, args...)
	}
	cmd := exec.Command("nix-store", args...)
	cmd.Env = l.HermeticEnv()
	if err := cmd.Run(); err != nil {
		return nil
	}
	raw, err := os.ReadFile(l.storeDrvPath(storeDir, drvPath))
	if err != nil {
		return nil
	}
	return raw
}

func (l Layout) storeDrvPath(storeDir, logicalDrvPath string) string {
	base := strings.TrimPrefix(logicalDrvPath, "/nix/store/")
	if storeDir == "" {
		storeDir = "/nix/store"
	}
	return filepath.Join(storeDir, base)
}

// InstantiateStrictInStore materialises (builtins.derivationStrict attrs) into storeDir.
func (l Layout) InstantiateStrictInStore(t *testing.T, storeDir, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	expr := fmt.Sprintf(`(builtins.derivationStrict %s)`, strictArg)
	args := append(append([]string{}, l.instantiateArgsWithStore(storeDir)...), "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			t.Fatalf("nix-instantiate: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	return strings.Trim(strings.TrimSpace(string(out)), `"`)
}

// EvalStrictOutPath evaluates (builtins.derivationStrict attrs).out hermetically (raw string).
func (l Layout) EvalStrictOutPath(t *testing.T, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(builtins.derivationStrict %s).out`, strictArg)
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--raw", "-E", pathExpr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			t.Fatalf("nix-instantiate: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// EvalStrictDrvPath evaluates (builtins.derivationStrict attrs).drvPath hermetically.
func (l Layout) EvalStrictDrvPath(t *testing.T, attrs string) string {
	return l.EvalStrictDrvPathInStore(t, "", attrs)
}

// EvalStrictDrvPathInStore is like EvalStrictDrvPath but uses nix-instantiate --store.
func (l Layout) EvalStrictDrvPathInStore(t *testing.T, storeDir, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(builtins.derivationStrict %s).drvPath`, strictArg)
	args := append(append([]string{}, l.instantiateArgsWithStore(storeDir)...), "--eval", "--json", "-E", pathExpr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			t.Fatalf("nix-instantiate: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	s, err := decodeNixInstantiateJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// ReadStoreDrvPath reads a .drv file from a logical store path (honours --store layout).
func (l Layout) ReadStoreDrvPath(t *testing.T, storeDir, logicalDrvPath string) []byte {
	t.Helper()
	base := strings.TrimPrefix(logicalDrvPath, "/nix/store/")
	if storeDir == "" {
		storeDir = "/nix/store"
	}
	path := filepath.Join(storeDir, base)
	if strings.HasSuffix(path, ".drv") {
		// already basename
	} else if !strings.HasSuffix(base, ".drv") {
		path += ".drv"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
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
