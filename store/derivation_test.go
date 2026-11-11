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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/connerohnesorge/goonix/store"
)

func TestReadDerivationDrvPath(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv"))
	if err != nil {
		t.Fatal(err)
	}
	drv, err := store.ReadDerivation(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.DrvPath(drv)
	if err != nil {
		t.Fatal(err)
	}
	want := "/nix/store/m5j1yp47lw1psd9n6bzina1167abbprr-bash44-023.drv"
	if got != want {
		t.Fatalf("DrvPath = %q, want %q", got, want)
	}
}

func TestDrvPathCustomStoreDir(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "4wvvbi4jwn0prsdxb7vs673qa5h9gr7x-foo.drv"))
	if err != nil {
		t.Fatal(err)
	}
	drv, err := store.ReadDerivation(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.Config{StoreDir: "/goonix/store"}
	got, err := store.DrvPathWithConfig(drv, cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := "/goonix/store/4wvvbi4jwn0prsdxb7vs673qa5h9gr7x-foo.drv"
	if got != want {
		t.Fatalf("DrvPathWithConfig = %q, want %q", got, want)
	}
}
