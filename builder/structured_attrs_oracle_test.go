package builder_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestStructuredAttrsBuildMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("nix-build"); err != nil {
		t.Skip("no nix-build")
	}
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("no nix-store")
	}

	script := `
      out=$0
      {
        while IFS= read -r line || [ -n "$line" ]; do
          printf '%s\n' "$line"
        done < "$NIX_ATTRS_JSON_FILE"
        printf '%s\n' '---sh---'
        while IFS= read -r line || [ -n "$line" ]; do
          printf '%s\n' "$line"
        done < "$NIX_ATTRS_SH_FILE"
      } > "$out"
    `
	echo := `builtins.derivationStrict {
	  __structuredAttrs = true;
	  name = "sa-echo";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  greeting = "hello";
	  flag = true;
	  n = 3;
	  items = [ "a" "b" ];
	  outputChecks = { out = { maxSize = 1000000; }; };
	  unsafeDiscardReferences = { out = false; };
	  args = [ "-c" ''` + script + `'' (builtins.placeholder "out") ];
	}`
	refs := `let
	  dep = builtins.derivationStrict {
	    name = "sa-dep";
	    system = "x86_64-linux";
	    builder = "/bin/sh";
	    args = [ "-c" "printf hi > $out" ];
	  };
	in builtins.derivationStrict {
	  __structuredAttrs = true;
	  name = "sa-refs";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  exportReferencesGraph = { deps = [ dep.out ]; };
	  args = [ "-c" ''` + script + `'' (builtins.placeholder "out") ];
	}`

	lay := oracle.MustLayout(t)
	wantEcho := lay.EvalRaw(t, "("+echo+").out")
	wantRefs := lay.EvalRaw(t, "("+refs+").out")

	goRoot := t.TempDir()
	st, err := local.OpenStore(goRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	goEcho, err := buildStrictEval(t, st, echo)
	if err != nil {
		t.Fatal(err)
	}
	goRefs, err := buildStrictEval(t, st, refs)
	if err != nil {
		t.Fatal(err)
	}
	if goEcho != wantEcho || goRefs != wantRefs {
		t.Fatalf("out paths echo go=%s nix=%s refs go=%s nix=%s", goEcho, wantEcho, goRefs, wantRefs)
	}

	nixEchoRoot := t.TempDir()
	nixEcho := realiseStructured(t, nixEchoRoot, echo)
	if nixEcho != wantEcho {
		t.Fatalf("nix realise echo %s eval %s", nixEcho, wantEcho)
	}
	compareNarDump(t, nixEchoRoot, nixEcho, goRoot, goEcho)

	nixRefsRoot := t.TempDir()
	nixRefs := realiseStructured(t, nixRefsRoot, refs)
	if nixRefs != wantRefs {
		t.Fatalf("nix realise refs %s eval %s", nixRefs, wantRefs)
	}
	compareNarDump(t, nixRefsRoot, nixRefs, goRoot, goRefs)

	// unsafeDiscardReferences drops the scanned reference to the dependency.
	discard := `let
	  dep = builtins.derivationStrict {
	    name = "sa-dep";
	    system = "x86_64-linux";
	    builder = "/bin/sh";
	    args = [ "-c" "printf hi > $out" ];
	  };
	in builtins.derivationStrict {
	  __structuredAttrs = true;
	  name = "sa-discard";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  unsafeDiscardReferences = { out = true; };
	  args = [ "-c" "printf '%s\n' '${dep.out}' > \"\$0\"" (builtins.placeholder "out") ];
	}`
	goDiscard, err := buildStrictEval(t, st, discard)
	if err != nil {
		t.Fatal(err)
	}
	refsGot, err := st.QueryReferences(goDiscard)
	if err != nil {
		t.Fatal(err)
	}
	if len(refsGot) != 0 {
		t.Fatalf("discarded output still references %v", refsGot)
	}
	nixDiscardRoot := t.TempDir()
	nixDiscard := realiseStructured(t, nixDiscardRoot, discard)
	if nixDiscard != goDiscard {
		t.Fatalf("discard out go=%s nix=%s", goDiscard, nixDiscard)
	}
	cmd := exec.Command("nix-store", "--store", oracle.LocalStoreURI(nixDiscardRoot), "-q", "--references", nixDiscard)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 && string(out) != "\n" {
		t.Fatalf("nix references after discard: %q", out)
	}
	compareNarDump(t, nixDiscardRoot, nixDiscard, goRoot, goDiscard)

	tooBig := `builtins.derivationStrict {
	  __structuredAttrs = true;
	  name = "sa-toobig";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  outputChecks = { out = { maxSize = 1; }; };
	  args = [ "-c" "printf hi > \"\$0\"" (builtins.placeholder "out") ];
	}`
	_, goErr := buildStrictEval(t, st, tooBig)
	if goErr == nil {
		t.Fatal("expected maxSize failure")
	}
	nixRoot := t.TempDir()
	nixErr := realiseStructuredErr(t, nixRoot, tooBig)
	for _, frag := range []string{"is too large at", "limit is 1 bytes"} {
		if !strings.Contains(goErr.Error(), frag) || !strings.Contains(nixErr, frag) {
			t.Fatalf("maxSize error\ngo=%s\nnix=%s", goErr, nixErr)
		}
	}

	selfRef := `builtins.derivationStrict {
	  __structuredAttrs = true;
	  name = "sa-selfref";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  outputChecks = { out = { allowedReferences = [ ]; }; };
	  args = [ "-c" "printf %s \"$0\" > \"$0\"" (builtins.placeholder "out") ];
	}`
	_, goSelf := buildStrictEval(t, st, selfRef)
	if goSelf == nil {
		t.Fatal("allowedReferences = [] must reject an output that contains its own path")
	}
	nixSelf := realiseStructuredErr(t, t.TempDir(), selfRef)
	frag := "is not allowed to refer to the following paths"
	if !strings.Contains(goSelf.Error(), frag) || !strings.Contains(nixSelf, frag) {
		t.Fatalf("self-reference check\ngo=%s\nnix=%s", goSelf, nixSelf)
	}
}

func realiseStructuredErr(t *testing.T, storeRoot, expr string) string {
	t.Helper()
	lay := oracle.MustLayout(t)
	inst := exec.Command("nix-instantiate", append(lay.InstantiateArgs(),
		"--store", oracle.LocalStoreURI(storeRoot),
		"-E", expr,
	)...)
	inst.Env = lay.HermeticEnv()
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("nix-instantiate: %v\n%s", err, out)
	}
	drv := strings.Trim(lay.EvalRaw(t, "("+expr+").drvPath"), `"`)
	cmd := exec.Command("nix-store", append([]string{
		"--store", oracle.LocalStoreURI(storeRoot),
		"--realise", drv,
	}, oracle.UnprivilegedBuildArgs()...)...)
	cmd.Env = oracle.BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected nix-store to fail:\n%s", out)
	}
	return string(out)
}

// realiseStructured instantiates a derivationStrict expression into an isolated
// store and builds it. nix-build does not realise a derivationStrict attrset.
func realiseStructured(t *testing.T, storeRoot, expr string) string {
	t.Helper()
	lay := oracle.MustLayout(t)
	inst := exec.Command("nix-instantiate", append(lay.InstantiateArgs(),
		"--store", oracle.LocalStoreURI(storeRoot),
		"-E", expr,
	)...)
	inst.Env = lay.HermeticEnv()
	if out, err := inst.CombinedOutput(); err != nil {
		t.Fatalf("nix-instantiate: %v\n%s", err, out)
	}
	drv := strings.Trim(lay.EvalRaw(t, "("+expr+").drvPath"), `"`)
	cmd := exec.Command("nix-store", append([]string{
		"--store", oracle.LocalStoreURI(storeRoot),
		"--realise", drv,
	}, oracle.UnprivilegedBuildArgs()...)...)
	cmd.Env = oracle.BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-store --realise %s: %v\n%s", drv, err, out)
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
