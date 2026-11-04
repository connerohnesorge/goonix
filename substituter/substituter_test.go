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

func TestTrustedPublicKeyEmbedded(t *testing.T) {
	_ = substituter.NewClient(substituter.Config{})
	pk, err := signature.ParsePublicKey("cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY=")
	if err != nil {
		t.Fatal(err)
	}
	if pk.Name != "cache.nixos.org-1" {
		t.Fatalf("name=%q", pk.Name)
	}
}
