// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval_test

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/connerohnesorge/goonix/internal/oracle"
)

func TestFetchTarballSymlinkMatchesNix(t *testing.T) {
	dir := t.TempDir()
	tarPath := filepath.Join(dir, "with-link.tar.gz")
	writeSymlinkTarball(t, tarPath)
	raw := "file://" + tarPath
	fetchTar := fmt.Sprintf("builtins.fetchTarball %q", raw)
	tree := fmt.Sprintf("builtins.fetchTree { type = \"tarball\"; url = %q; }", raw)
	goTar := evalPkgsExprString(t, fetchTar)
	goOut := evalPkgsExprString(t, "("+tree+").outPath")
	goNar := evalPkgsExprString(t, "("+tree+").narHash")
	if goTar != goOut {
		t.Fatalf("fetchTarball and fetchTree disagree\nfetchTarball=%s\nfetchTree=%s", goTar, goOut)
	}
	if goNar == "" {
		t.Fatal("empty narHash")
	}
	oracle.RequireNix224(t)
	lay := oracle.MustLayout(t)
	if want := lay.EvalRaw(t, fetchTar); want != goTar {
		t.Fatalf("fetchTarball\nnix=%s\ngo=%s", want, goTar)
	}
	if want := lay.EvalRaw(t, "("+tree+").outPath"); want != goOut {
		t.Fatalf("fetchTree outPath\nnix=%s\ngo=%s", want, goOut)
	}
	if want := lay.EvalRaw(t, "("+tree+").narHash"); want != goNar {
		t.Fatalf("fetchTree narHash\nnix=%s\ngo=%s", want, goNar)
	}
}

func writeSymlinkTarball(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	body := []byte("hi\n")
	when := time.Unix(1_700_000_000, 0)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "top/file",
		Mode:     0o644,
		Size:     int64(len(body)),
		Typeflag: tar.TypeReg,
		ModTime:  when,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.WriteHeader(&tar.Header{
		Name:     "top/link",
		Mode:     0o777,
		Typeflag: tar.TypeSymlink,
		Linkname: "file",
		ModTime:  when,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}
