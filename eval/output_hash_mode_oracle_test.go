package eval_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/eval"
	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestOutputHashModeMatchesNix224(t *testing.T) {
	const hash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	const gitHash = "sha1-AAAAAAAAAAAAAAAAAAAAAAAAAAA="
	flat := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "flat"; outputHashAlgo = "sha256"; outputHash = "` + hash + `"; }`
	text := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "text"; outputHashAlgo = "sha256"; outputHash = "` + hash + `"; }`
	nar := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "nar"; outputHashAlgo = "sha256"; outputHash = "` + hash + `"; }`
	recursive := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "recursive"; outputHashAlgo = "sha256"; outputHash = "` + hash + `"; }`
	git := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "git"; outputHashAlgo = "sha1"; outputHash = "` + gitHash + `"; }`
	custom := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "custom"; outputHashAlgo = "sha256"; outputHash = "` + hash + `"; }`

	goFlat := outPathOf(t, eval.NewEvaluator(), flat)
	goText := outPathOf(t, eval.NewEvaluator().WithExperimental("dynamic-derivations"), text)
	goNar := outPathOf(t, eval.NewEvaluator(), nar)
	goRec := outPathOf(t, eval.NewEvaluator(), recursive)
	goGit := outPathOf(t, eval.NewEvaluator().WithExperimental("git-hashing"), git)
	if goText == goFlat {
		t.Fatalf("text out path %s equals flat; text is not a distinct ingestion method", goText)
	}
	if goNar != goRec {
		t.Fatalf("nar %s != recursive %s", goNar, goRec)
	}
	_, err := evalRaw(t, eval.NewEvaluator(), "builtins.derivationStrict "+custom)
	if err == nil || !strings.Contains(err.Error(), "invalid value 'custom' for 'outputHashMode' attribute") {
		t.Fatalf("custom mode: %v", err)
	}
	_, err = evalRaw(t, eval.NewEvaluator(), "builtins.derivationStrict "+text)
	if err == nil || !strings.Contains(err.Error(), "experimental Nix feature 'dynamic-derivations' is disabled") {
		t.Fatalf("text without feature: %v", err)
	}
	_, err = evalRaw(t, eval.NewEvaluator(), "builtins.derivationStrict "+git)
	if err == nil || !strings.Contains(err.Error(), "experimental Nix feature 'git-hashing' is disabled") {
		t.Fatalf("git without feature: %v", err)
	}

	oracle.RequireNix224(t)
	nixFlat := nixOut224(t, "", flat)
	nixText := nixOut224(t, "dynamic-derivations", text)
	nixNar := nixOut224(t, "", nar)
	nixRec := nixOut224(t, "", recursive)
	nixGit := nixOut224(t, "git-hashing", git)
	if goFlat != nixFlat || goText != nixText || goNar != nixNar || goRec != nixRec || goGit != nixGit {
		t.Fatalf("out paths\n flat go=%s nix=%s\n text go=%s nix=%s\n nar go=%s nix=%s\n rec go=%s nix=%s\n git go=%s nix=%s",
			goFlat, nixFlat, goText, nixText, goNar, nixNar, goRec, nixRec, goGit, nixGit)
	}
	_, nerr := nixEvalXP(t, "", "(builtins.derivationStrict "+custom+").out")
	if nerr == nil || !strings.Contains(nerr.Error(), "invalid value 'custom' for 'outputHashMode' attribute") {
		t.Fatalf("nix custom: %v", nerr)
	}
	_, nerr = nixEvalXP(t, "", "(builtins.derivationStrict "+text+").out")
	if nerr == nil || !strings.Contains(nerr.Error(), "experimental Nix feature 'dynamic-derivations' is disabled") {
		t.Fatalf("nix text feature: %v", nerr)
	}
	_, nerr = nixEvalXP(t, "git-hashing", `(builtins.derivationStrict { name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "git"; outputHashAlgo = "sha256"; outputHash = "`+hash+`"; }).out`)
	if nerr == nil || !strings.Contains(nerr.Error(), "Git file ingestion must use SHA-1 hash") {
		t.Fatalf("nix git sha256: %v", nerr)
	}
	_, gerr := evalRaw(t, eval.NewEvaluator().WithExperimental("git-hashing"), `builtins.derivationStrict { name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "git"; outputHashAlgo = "sha256"; outputHash = "`+hash+`"; }`)
	if gerr == nil || !strings.Contains(gerr.Error(), "Git file ingestion must use SHA-1 hash") {
		t.Fatalf("go git sha256: %v", gerr)
	}

	// Nix 2.24.9 aborts in makeFixedOutputPathFromCA when a text hash is not
	// SHA-256. goonix returns an error instead of aborting the process.
	sha1Text := `{ name = "x"; system = "x86_64-linux"; builder = "/bin/sh"; outputHashMode = "text"; outputHashAlgo = "sha1"; outputHash = "` + gitHash + `"; }`
	_, nerr = nixEvalXP(t, "dynamic-derivations", "(builtins.derivationStrict "+sha1Text+").out")
	if nerr == nil || !strings.Contains(nerr.Error(), "Assertion") || !strings.Contains(nerr.Error(), "SHA256") {
		t.Fatalf("nix text sha1: %v", nerr)
	}
	_, gerr = evalRaw(t, eval.NewEvaluator().WithExperimental("dynamic-derivations"), "builtins.derivationStrict "+sha1Text)
	if gerr == nil || !strings.Contains(gerr.Error(), "text output hash must be sha256") {
		t.Fatalf("go text sha1: %v", gerr)
	}
}

func outPathOf(t *testing.T, ev *eval.Evaluator, attrs string) string {
	t.Helper()
	v := mustEval(t, ev, "(builtins.derivationStrict "+attrs+").out")
	if v.Kind != eval.VKString {
		t.Fatalf("out kind %v", v.Kind)
	}
	return v.String
}

func nixOut224(t *testing.T, extra, attrs string) string {
	t.Helper()
	s, err := nixEvalXP(t, extra, "(builtins.derivationStrict "+attrs+").out")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func nixEvalXP(t *testing.T, extra, expr string) (string, error) {
	t.Helper()
	features := "nix-command"
	if extra != "" {
		features += " " + extra
	}
	cmd := exec.Command("nix-instantiate",
		"--option", "experimental-features", features,
		"--option", "substituters", "",
		"--eval", "--json", "-E", expr)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=/tmp"}
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s", strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	s := strings.TrimSpace(string(out))
	var decoded string
	if jsonErr := json.Unmarshal([]byte(s), &decoded); jsonErr == nil {
		return decoded, nil
	}
	return s, nil
}
