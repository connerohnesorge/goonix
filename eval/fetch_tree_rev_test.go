// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/connerohnesorge/goonix/parser"
)

func TestFetchTreeStringRevIsHonoured(t *testing.T) {
	rev := "0123456789abcdef0123456789abcdef01234567"
	missing := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	ball := tinyGzipTar(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, missing) || strings.Contains(r.URL.RawQuery, missing) {
			http.Error(w, "missing", http.StatusNotFound)
			return
		}
		if strings.HasSuffix(r.URL.Path, ".tar.gz") || strings.Contains(r.URL.Path, "/archive") {
			w.Write(ball)
			return
		}
		if strings.Contains(r.URL.Path, "/repository/commits") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`[{"id":"` + rev + `"}]`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	oldGL := gitlabAPI
	gitlabAPI = srv.URL
	t.Cleanup(func() {
		gitlabAPI = oldGL
	})

	ev := NewEvaluator().WithEffects(ChrootIOEffects{PhysRoot: t.TempDir()})
	got, err := ev.fetchTreeURL(parser.Pos{}, "gitlab:o/r/"+rev)
	if err != nil {
		t.Fatal(err)
	}
	assertRev(t, ev, got, rev)
	_, err = ev.fetchTreeURL(parser.Pos{}, "gitlab:o/r/"+missing)
	if err == nil || !strings.Contains(err.Error(), "HTTP error 404") {
		t.Fatalf("missing gitlab rev: %v", err)
	}
}

func assertRev(t *testing.T, ev *Evaluator, v *Value, rev string) {
	t.Helper()
	fv, err := ev.ForceValue(v)
	if err != nil {
		t.Fatal(err)
	}
	revV := fv.Attrs.Binds[ev.Intern.Intern("rev")]
	shortV := fv.Attrs.Binds[ev.Intern.Intern("shortRev")]
	if revV == nil || shortV == nil {
		t.Fatalf("missing rev/shortRev in %#v", fv)
	}
	if revV.Value.String != rev || shortV.Value.String != rev[:7] {
		t.Fatalf("rev=%q short=%q", revV.Value.String, shortV.Value.String)
	}
}

func tinyGzipTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	body := []byte("hi\n")
	hdr := &tar.Header{
		Name:    "src/hello",
		Mode:    0o644,
		Size:    int64(len(body)),
		ModTime: time.Unix(0, 0),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
