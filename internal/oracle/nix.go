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
	if !strings.HasSuffix(base, ".drv") {
		base += ".drv"
	}
	if storeDir == "" {
		return filepath.Join("/nix/store", base)
	}
	return filepath.Join(storeDir, "nix", "store", base)
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
	args := append(append([]string{}, l.instantiateArgsWithStore("")...), "--eval", "--json", "-E", pathExpr)
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

// EvalImportJSON evaluates (import absPath {}) with hermetic -I flags and returns a JSON object.
func (l Layout) EvalImportJSON(t *testing.T, absPath string) map[string]any {
	t.Helper()
	quoted := `"` + strings.ReplaceAll(absPath, `"`, `\"`) + `"`
	expr := "import " + quoted + " {}"
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--strict", "--json", "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			t.Fatalf("nix-instantiate: %v\n%s", err, ee.Stderr)
		}
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		t.Fatalf("json: %v\n%s", err, out)
	}
	return v
}

// EvalImportJSONAllowError is like EvalImportJSON but returns errors instead of failing the test.
func (l Layout) EvalImportJSONAllowError(absPath string) (map[string]any, error) {
	quoted := `"` + strings.ReplaceAll(absPath, `"`, `\"`) + `"`
	expr := "import " + quoted + " {}"
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--strict", "--json", "-E", expr)
	cmd := exec.Command("nix-instantiate", args...)
	cmd.Env = l.HermeticEnv()
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("nix-instantiate: %w\n%s", err, ee.Stderr)
		}
		return nil, fmt.Errorf("nix-instantiate: %w", err)
	}
	var v map[string]any
	if err := json.Unmarshal(out, &v); err != nil {
		return nil, fmt.Errorf("json: %w\n%s", err, out)
	}
	return v, nil
}

// EvalDerivationDrvPath evaluates (derivation attrs).drvPath hermetically (Nix derivation, not strict).
func (l Layout) EvalDerivationDrvPath(t *testing.T, attrs string) string {
	return l.EvalDerivationDrvPathInStore(t, "", attrs)
}

// EvalDerivationDrvPathInStore is like EvalDerivationDrvPath with --store.
func (l Layout) EvalDerivationDrvPathInStore(t *testing.T, storeDir, attrs string) string {
	t.Helper()
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(derivation %s).drvPath`, strictArg)
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

// EvalDerivationDrvPathAllowError returns an error when nix-instantiate fails.
func (l Layout) EvalDerivationDrvPathAllowError(attrs string) (string, error) {
	strictArg := attrs
	if strings.HasPrefix(strings.TrimSpace(attrs), "let") {
		strictArg = "(" + attrs + ")"
	}
	pathExpr := fmt.Sprintf(`(derivation %s).drvPath`, strictArg)
	args := append(append([]string{}, l.InstantiateArgs()...), "--eval", "--json", "-E", pathExpr)
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
	path := l.storeDrvPath(storeDir, logicalDrvPath)
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

// LocalStoreURI is the --store argument for an isolated store under storeRoot (physical layout storeRoot/nix/store/…).
func LocalStoreURI(storeRoot string) string {
	return "local?root=" + storeRoot
}

// UnprivilegedBuildArgs are nix-build flags for an isolated local?root store on unprivileged hosts.
// Sandbox must be enabled so $out is mapped into the store mount; sandbox false writes to host /nix/store and fails on CI.
func UnprivilegedBuildArgs() []string {
	return []string{
		"--option", "sandbox", "true",
		"--option", "build-users-group", "",
	}
}

// BuildCmdEnv returns an environment for nix-build against a temp local store.
// Host NIX_STORE (e.g. /nix/store from install-nix-action) must not leak in or builds write to the daemon store.
func BuildCmdEnv(storeRoot string) []string {
	lay, err := FindLayout()
	var base []string
	if err != nil {
		base = []string{
			"PATH=" + os.Getenv("PATH"),
			"HOME=" + os.Getenv("HOME"),
		}
	} else {
		base = lay.HermeticEnv()
	}
	out := make([]string, 0, len(base)+1)
	for _, kv := range base {
		if strings.HasPrefix(kv, "NIX_STORE=") {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "NIX_STATE_DIR="+filepath.Join(storeRoot, "nix", "var", "nix"))
	return out
}

// NixBuildDerivation builds derivation attrs in storeRoot and returns the logical output path.
func NixBuildDerivation(t *testing.T, storeRoot, attrs string) string {
	t.Helper()
	cmd := exec.Command("nix-build", append([]string{
		"--store", LocalStoreURI(storeRoot),
	}, append(UnprivilegedBuildArgs(), "-E", "derivation "+attrs)...)...)
	cmd.Env = BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-build in temp store: %v\n%s", err, out)
	}
	text := strings.TrimSpace(string(out))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/nix/store/") && !strings.HasSuffix(line, ".drv") {
			return line
		}
	}
	t.Fatalf("nix-build output missing out path:\n%s", out)
	return ""
}

// NixBuildExpr builds a Nix expression with nix-build in storeRoot and returns the logical output path.
func NixBuildExpr(t *testing.T, storeRoot, expr string) string {
	t.Helper()
	cmd := exec.Command("nix-build", append([]string{
		"--store", LocalStoreURI(storeRoot),
		"--no-out-link",
	}, append(UnprivilegedBuildArgs(), "-E", expr)...)...)
	cmd.Env = BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-build -E: %v\n%s", err, out)
	}
	text := strings.TrimSpace(string(out))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/nix/store/") && !strings.HasSuffix(line, ".drv") {
			return line
		}
	}
	t.Fatalf("nix-build output missing out path:\n%s", out)
	return ""
}

// NixBuildStrictOutPath instantiates (builtins.derivationStrict attrs), realises it in storeRoot,
// and returns the logical "out" store path (supports let … in attrs).
func NixBuildStrictOutPath(t *testing.T, storeRoot, attrs string) string {
	t.Helper()
	lay := MustLayout(t)
	drvPath := lay.InstantiateStrictInStore(t, storeRoot, attrs)
	cmd := exec.Command("nix-store", append([]string{
		"--store", LocalStoreURI(storeRoot),
		"--realise", drvPath,
	})...)
	cmd.Env = BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-store --realise %s: %v\n%s", drvPath, err, out)
	}
	var last string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/nix/store/") && !strings.HasSuffix(line, ".drv") {
			last = line
		}
	}
	if last == "" {
		t.Fatalf("nix-store --realise missing out path:\n%s", out)
	}
	return last
}
