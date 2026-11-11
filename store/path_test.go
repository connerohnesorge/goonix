// Copyright 2026 The goonix authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package store_test

import (
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func TestParseAbsoluteDefault(t *testing.T) {
	const p = "/nix/store/m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv"
	cfg := store.Config{}
	sp, err := cfg.ParseAbsolute(p)
	if err != nil {
		t.Fatal(err)
	}
	if sp.Name != "bash44-023.drv" {
		t.Fatalf("name = %q", sp.Name)
	}
}

func TestCustomStoreDir(t *testing.T) {
	cfg := store.Config{StoreDir: "/custom/store"}
	const rel = "m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv"
	want := filepath.Join("/custom/store", rel)
	got := cfg.RewriteDefaultStoreDir("/nix/store/" + rel)
	if got != want {
		t.Fatalf("Rewrite = %q, want %q", got, want)
	}
	sp, err := store.ParseRelative(rel)
	if err != nil {
		t.Fatal(err)
	}
	if abs := cfg.Absolute(sp); abs != want {
		t.Fatalf("Absolute = %q, want %q", abs, want)
	}
}

func TestMakeAbsolute(t *testing.T) {
	cfg := store.Config{}
	digest := spDigest(t, "m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv")
	got, err := cfg.MakeAbsolute("bash44-023.drv", digest)
	if err != nil {
		t.Fatal(err)
	}
	want := "/nix/store/m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv"
	if got != want {
		t.Fatalf("MakeAbsolute = %q, want %q", got, want)
	}
}

func spDigest(t *testing.T, rel string) []byte {
	t.Helper()
	sp, err := store.ParseRelative(rel)
	if err != nil {
		t.Fatal(err)
	}
	return sp.Digest
}
