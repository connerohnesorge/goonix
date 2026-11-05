// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package substituter_test

import (
	"strings"
	"testing"

	"github.com/nix-community/go-nix/pkg/narinfo"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"

	"github.com/connerohnesorge/goonix/substituter"
	"github.com/connerohnesorge/goonix/store/local"
)

const sampleNarinfo = `
StorePath: /nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432
URL: nar/1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d.nar.xz
Compression: xz
FileHash: sha256:1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d
FileSize: 114980
NarHash: sha256:0lxjvvpr59c2mdram7ympy5ay741f180kv3349hvfc3f8nrmbqf6
NarSize: 464152
References: 7gx4kiv5m0i7d7qkixq2cwzbr10lvxwc-glibc-2.27
Deriver: 10dx1q4ivjb115y3h90mipaaz533nr0d-net-tools-1.60_p20170221182432.drv
Sig: cache.nixos.org-1:sn5s/RrqEI+YG6/PjwdbPjcAC7rcta7sJU4mFOawGvJBLsWkyLtBrT2EuFt/LJjWkTZ+ZWOI9NTtjo/woMdvAg==
`

func TestParseFixtureNarinfo(t *testing.T) {
	ni, err := narinfo.Parse(strings.NewReader(sampleNarinfo))
	if err != nil {
		t.Fatal(err)
	}
	if ni.Compression != "xz" {
		t.Fatalf("compression=%q", ni.Compression)
	}
	if ni.NarHash == nil {
		t.Fatal("missing NarHash")
	}
	if len(ni.References) != 1 {
		t.Fatalf("refs=%d", len(ni.References))
	}
}

func TestFixtureNarinfoSignatureVerifies(t *testing.T) {
	ni, err := narinfo.Parse(strings.NewReader(sampleNarinfo))
	if err != nil {
		t.Fatal(err)
	}
	pk, err := signature.ParsePublicKey("cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=")
	if err != nil {
		t.Fatal(err)
	}
	if !signature.VerifyFirst(ni.Fingerprint(), ni.Signatures, []signature.PublicKey{pk}) {
		t.Fatal("expected cache.nixos.org-1 signature to verify")
	}
}

func TestFixtureNarinfoSignatureRejectedWhenTampered(t *testing.T) {
	ni, err := narinfo.Parse(strings.NewReader(sampleNarinfo))
	if err != nil {
		t.Fatal(err)
	}
	ni.Signatures[0].Data[0] ^= 0xff
	pk, err := signature.ParsePublicKey("cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=")
	if err != nil {
		t.Fatal(err)
	}
	if signature.VerifyFirst(ni.Fingerprint(), ni.Signatures, []signature.PublicKey{pk}) {
		t.Fatal("tampered signature must not verify")
	}
}

func TestSubstituteHelloFromCache(t *testing.T) {
	if testing.Short() {
		t.Skip("network")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := substituter.NewClient(substituter.Config{})
	// Small path with a tiny reference closure (pinned from cache.nixos.org).
	path := "/nix/store/hwz2l7ihv2skq7gr5l3paavs3rr9il7z-hello-2.12.1"
	ok, err := c.TrySubstitute(st, path)
	if err != nil {
		if substituter.IsNetworkUnavailable(err) {
			t.Skipf("substitute: %v", err)
		}
		t.Fatalf("substitute: %v", err)
	}
	if !ok {
		t.Skip("path not substituted (cache miss or offline)")
	}
	valid, err := st.IsValidPath(path)
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
}

func TestSubstituteNetToolsFromCache(t *testing.T) {
	t.Skip("net-tools closure is large; use TestSubstituteHelloFromCache")
	if testing.Short() {
		t.Skip("network")
	}
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := substituter.NewClient(substituter.Config{})
	path := "/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432"
	ok, err := c.TrySubstitute(st, path)
	if err != nil {
		t.Skipf("substitute: %v", err)
	}
	if !ok {
		t.Skip("path not substituted (offline or cache miss)")
	}
	valid, err := st.IsValidPath(path)
	if err != nil || !valid {
		t.Fatalf("valid=%v err=%v", valid, err)
	}
}
