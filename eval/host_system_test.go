package eval

import "testing"

func TestHostSystemMatchesNixSpelling(t *testing.T) {
	cases := []struct{ arch, os, want string }{
		{"amd64", "linux", "x86_64-linux"},
		{"arm64", "linux", "aarch64-linux"},
		{"arm64", "darwin", "aarch64-darwin"},
		{"amd64", "darwin", "x86_64-darwin"},
		{"amd64", "windows", "x86_64-windows"},
		{"arm64", "windows", "aarch64-windows"},
	}
	for _, c := range cases {
		if got := hostSystem(c.arch, c.os); got != c.want {
			t.Errorf("hostSystem(%s, %s) = %s, want %s", c.arch, c.os, got, c.want)
		}
	}
}
