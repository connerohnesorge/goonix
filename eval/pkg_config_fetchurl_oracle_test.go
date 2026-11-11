package eval_test

import (
	"bytes"
	"os"
	"testing"
)

// Oracle: nixpkgs fetchurl for pkg-config-0.29.2.tar.gz uses nix-base32 outputHash in env.
func TestPkgConfigFetchurlNixBase32OutputHashMatchesNix(t *testing.T) {
	if _, err := os.Stat("/nix/store"); err != nil {
		t.Skip("no /nix/store")
	}
	attrs := `{
	  name = "pkg-config-0.29.2.tar.gz";
	  system = "builtin";
	  builder = "builtin:fetchurl";
	  url = "https://pkg-config.freedesktop.org/releases/pkg-config-0.29.2.tar.gz";
	  urls = [ "https://pkg-config.freedesktop.org/releases/pkg-config-0.29.2.tar.gz" ];
	  outputHash = "14fmwzki1rlz8bs2p810lk6jqdxsk966d8drgsjmi54cd00rrikg";
	  outputHashAlgo = "sha256";
	  outputHashMode = "flat";
	  preferLocalBuild = true;
	  impureEnvVars = [ "http_proxy" "https_proxy" "ftp_proxy" "all_proxy" "no_proxy" ];
	}`
	nixPath, nixATerm := nixDerivationStrictOracle(t, "", attrs)
	goPath, goATerm := goDerivationStrictOracle(t, "", attrs)
	if nixPath != goPath {
		t.Fatalf("drvPath mismatch\nnix=%q\ngo=%q", nixPath, goPath)
	}
	if len(nixATerm) > 0 && !bytes.Equal(nixATerm, goATerm) {
		t.Fatalf("ATerm mismatch\nnix=%s\ngo=%s", nixATerm, goATerm)
	}
}
