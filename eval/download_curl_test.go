// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadCheckedReportsHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := downloadChecked(srv.URL)
	if err == nil || !strings.Contains(err.Error(), "HTTP error 404") {
		t.Fatal(err)
	}
}
