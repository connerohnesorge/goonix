package store_test

import (
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

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
