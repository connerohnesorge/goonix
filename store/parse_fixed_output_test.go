package store_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func TestParseFixedOutputHashNixBase32PkgConfig(t *testing.T) {
	base16, drvAlgo, err := store.ParseFixedOutputHash(
		"14fmwzki1rlz8bs2p810lk6jqdxsk966d8drgsjmi54cd00rrikg",
		"sha256",
		"flat",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "6fc69c01688c9458a57eb9a1664c9aba372ccda420a02bf4429fe610e7e7d591"
	if base16 != want {
		t.Fatalf("base16=%q want %q", base16, want)
	}
	if drvAlgo != "sha256" {
		t.Fatalf("drvAlgo=%q", drvAlgo)
	}
}

func TestParseFixedOutputHashFetchurlSRI(t *testing.T) {
	base16, drvAlgo, err := store.ParseFixedOutputHash(
		"sha256-BsnhO99+sk1M62tZIFpPZ8LH5yExGWREMP6C+9FKCrs=",
		"",
		"flat",
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "06c9e13bdf7eb24d4ceb6b59205a4f67c2c7e7213119644430fe82fbd14a0abb"
	if base16 != want {
		t.Fatalf("base16=%q want %q", base16, want)
	}
	if drvAlgo != "sha256" {
		t.Fatalf("drvAlgo=%q", drvAlgo)
	}
}
