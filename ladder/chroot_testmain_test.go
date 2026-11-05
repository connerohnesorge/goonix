package ladder_test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "goonix-ladder-chroot-")
	if err != nil {
		os.Exit(1)
	}
	_ = os.Setenv("GOONIX_CHROOT_PHYS", root)
	code := m.Run()
	_ = os.RemoveAll(root)
	_ = os.Unsetenv("GOONIX_CHROOT_PHYS")
	os.Exit(code)
}
