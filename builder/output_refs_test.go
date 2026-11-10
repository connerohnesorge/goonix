package builder_test

import (
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/connerohnesorge/goonix/internal/oracle"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestOutputScanRecordsSelfAndSiblings(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	expr := `builtins.derivationStrict {
	  name = "sib";
	  outputs = [ "out" "dev" ];
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  args = [ "-c" "printf '%s\n' \"$out\" \"$dev\" > \"$out\"; printf ok > \"$dev\"" ];
	}`
	goRoot := t.TempDir()
	st, err := local.OpenStore(goRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	outPath, err := buildStrictEval(t, st, expr)
	if err != nil {
		t.Fatal(err)
	}
	refs, err := st.QueryReferences(outPath)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range refs {
		got[r] = true
	}
	if !got[outPath] {
		t.Fatalf("out refs %v do not include self %s", refs, outPath)
	}
	var sibling string
	for _, r := range refs {
		if r != outPath {
			sibling = r
		}
	}
	if sibling == "" {
		t.Fatalf("out refs %v do not include the dev sibling", refs)
	}
	devPath := sibling

	junk, err := st.AddToStoreText("junk", "delete-me\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddGCRoot("sib-out", outPath); err != nil {
		t.Fatal(err)
	}
	deleted, err := st.CollectGarbage()
	if err != nil {
		t.Fatal(err)
	}
	dropped := map[string]bool{}
	for _, p := range deleted {
		dropped[p] = true
	}
	if !dropped[junk] {
		t.Fatalf("deleted %v, want %s among them", deleted, junk)
	}
	for _, p := range []string{outPath, devPath} {
		ok, err := st.IsValidPath(p)
		if err != nil || !ok {
			t.Fatalf("%s valid=%v err=%v", p, ok, err)
		}
	}

	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("no nix-store")
	}
	nixRoot := t.TempDir()
	_ = realiseStructured(t, nixRoot, expr)
	nixRefs := nixQueryReferences(t, nixRoot, outPath)
	sort.Strings(refs)
	sort.Strings(nixRefs)
	if strings.Join(nixRefs, "\n") != strings.Join(refs, "\n") {
		t.Fatalf("references\ngo=%v\nnix=%v", refs, nixRefs)
	}
}

func nixQueryReferences(t *testing.T, storeRoot, path string) []string {
	t.Helper()
	cmd := exec.Command("nix-store",
		"--store", oracle.LocalStoreURI(storeRoot),
		"-q", "--references", path,
	)
	cmd.Env = oracle.BuildCmdEnv(storeRoot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("nix-store -q --references: %v\n%s", err, out)
	}
	var refs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			refs = append(refs, line)
		}
	}
	return refs
}
