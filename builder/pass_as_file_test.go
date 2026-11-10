package builder_test

import (
	"os/exec"
	"testing"

	"github.com/connerohnesorge/goonix/store/local"
)

func TestPassAsFileMatchesNix(t *testing.T) {
	if _, err := exec.LookPath("/bin/sh"); err != nil {
		t.Skip("no /bin/sh")
	}
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("no nix-store")
	}
	expr := `builtins.derivationStrict {
	  name = "paf";
	  system = "x86_64-linux";
	  builder = "/bin/sh";
	  passAsFile = "secret note";
	  secret = "line1\nquote\"\n";
	  note = "second\n";
	  args = [ "-c" "if [ -n \"\${secret-}\" ] || [ -n \"\${note-}\" ]; then printf leaked > \"$out\"; exit 1; fi; { while IFS= read -r line || [ -n \"$line\" ]; do printf '%s\n' \"$line\"; done < \"$secretPath\"; printf '|'; while IFS= read -r line || [ -n \"$line\" ]; do printf '%s\n' \"$line\"; done < \"$notePath\"; } > \"$out\"" ];
	}`
	goRoot := t.TempDir()
	st, err := local.OpenStore(goRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	goOut, err := buildStrictEval(t, st, expr)
	if err != nil {
		t.Fatal(err)
	}
	nixRoot := t.TempDir()
	nixOut := realiseStructured(t, nixRoot, expr)
	compareNarDump(t, nixRoot, nixOut, goRoot, goOut)
}
