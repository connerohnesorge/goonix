package substituter_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nix-community/go-nix/pkg/narinfo"

	"github.com/connerohnesorge/goonix/substituter"
	"github.com/connerohnesorge/goonix/store/local"
)

func TestNarInfoURLUsesHashOnly(t *testing.T) {
	c := substituter.NewClient(substituter.Config{CacheURL: "https://cache.example"})
	path := "/nix/store/00bgd045z0d4icpbc2yyz4gx48ak44la-net-tools-1.60_p20170221182432"
	u, err := c.NarInfoURL(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(u, "00bgd045z0d4icpbc2yyz4gx48ak44la.narinfo") {
		t.Fatalf("url=%q", u)
	}
	if strings.Contains(u, "net-tools") {
		t.Fatalf("url must not contain name suffix: %q", u)
	}
}

func TestClientRejectsMissingSignature(t *testing.T) {
	const unsigned = `
StorePath: /nix/store/abc12345placeholder-hello-1.0
URL: nar/x.nar.xz
Compression: xz
FileHash: sha256:1094wph9z4nwlgvsd53abfz8i117ykiv5dwnq9nnhz846s7xqd7d
FileSize: 1
NarHash: sha256:0lxjvvpr59c2mdram7ympy5ay741f180kv3349hvfc3f8nrmbqf6
NarSize: 1
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".narinfo") {
			_, _ = w.Write([]byte(unsigned))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := substituter.NewClient(substituter.Config{CacheURL: srv.URL})
	path := "/nix/store/abc12345placeholder-hello-1.0"
	ok, err := c.TrySubstitute(st, path)
	if err == nil && ok {
		t.Fatal("expected signature error")
	}
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestClientRejectsTamperedSignature(t *testing.T) {
	ni, err := narinfo.Parse(strings.NewReader(sampleNarinfo))
	if err != nil {
		t.Fatal(err)
	}
	ni.Signatures[0].Data[0] ^= 0xff
	body := ni.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".narinfo") {
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	st, err := local.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	c := substituter.NewClient(substituter.Config{CacheURL: srv.URL})
	path := ni.StorePath
	ok, err := c.TrySubstitute(st, path)
	if err == nil && ok {
		t.Fatal("expected bad signature to fail")
	}
	if err == nil {
		t.Fatal("expected signature verification error")
	}
}
