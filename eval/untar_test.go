package eval

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type tarEntry struct {
	name, body, link string
	typ              byte
}

func makeTar(t *testing.T, gz bool, entries ...tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o644, Linkname: e.link, Size: int64(len(e.body))}
		if e.typ != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if !gz {
		return buf.Bytes()
	}
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestUntarRejectsDotDot(t *testing.T) {
	root := t.TempDir()
	dest := filepath.Join(root, "a", "b", "unpacked")
	data := makeTar(t, true, tarEntry{name: "../../escaped", body: "x", typ: tar.TypeReg})
	if _, err := unpackTarball(data, dest); !errors.Is(err, errUnsafeTarEntry) {
		t.Fatalf("err = %v, want errUnsafeTarEntry", err)
	}
	if _, err := os.Stat(filepath.Join(root, "a", "escaped")); err == nil {
		t.Fatal("entry written outside dest")
	}
}

func TestUntarDoesNotWriteThroughSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(root, "unpacked")
	data := makeTar(t, true,
		tarEntry{name: "a", link: outside, typ: tar.TypeSymlink},
		tarEntry{name: "a/pwned", body: "x", typ: tar.TypeReg},
	)
	_, _ = unpackTarball(data, dest)
	if _, err := os.Stat(filepath.Join(outside, "pwned")); err == nil {
		t.Fatal("file written through an archive symlink")
	}
}

func TestUntarHardLinkAndSymlink(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "unpacked")
	data := makeTar(t, true,
		tarEntry{name: "src/f", body: "hello", typ: tar.TypeReg},
		tarEntry{name: "src/h", link: "src/f", typ: tar.TypeLink},
		tarEntry{name: "src/s", link: "f", typ: tar.TypeSymlink},
	)
	if _, err := unpackTarball(data, dest); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "src", "h"))
	if err != nil || string(b) != "hello" {
		t.Fatalf("hard link = %q, %v", b, err)
	}
	if l, err := os.Readlink(filepath.Join(dest, "src", "s")); err != nil || l != "f" {
		t.Fatalf("symlink = %q, %v", l, err)
	}
}

func TestUntarPlainTarAfterFailedGzip(t *testing.T) {
	// A plain tar is not gzip, so only the last attempt succeeds; dest must
	// hold exactly its entries.
	dest := filepath.Join(t.TempDir(), "unpacked")
	data := makeTar(t, false, tarEntry{name: "only", body: "1", typ: tar.TypeReg})
	if _, err := unpackTarball(data, dest); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(dest)
	if err != nil || len(ents) != 1 || ents[0].Name() != "only" {
		t.Fatalf("entries = %v, %v", ents, err)
	}
}
