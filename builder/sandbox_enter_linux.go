// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package builder

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func init() {
	if os.Getenv("GOONIX_SANDBOX_STAGE") == "" {
		return
	}
	os.Exit(sandboxMain())
}

type sandboxSpec struct {
	StorePhys string   `json:"storePhys"`
	BuildDir  string   `json:"buildDir"`
	Sentinel  string   `json:"sentinel"`
	Builder   string   `json:"builder"`
	Args      []string `json:"args"`
	Env       []string `json:"env"`
	Closure   []string `json:"closure"`
	Outputs   []string `json:"outputs"`
}

type sandboxExit int

func (e sandboxExit) Error() string { return fmt.Sprintf("builder exit %d", int(e)) }

func sandboxMain() int {
	err := sandboxSetup()
	if err == nil {
		return 0
	}
	if code, ok := err.(sandboxExit); ok {
		return int(code)
	}
	fmt.Fprintf(os.Stderr, "goonix sandbox: %s\n", err)
	return 1
}

func sandboxSetup() error {
	specPath := os.Getenv("GOONIX_SANDBOX_SPEC")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var spec sandboxSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return err
	}
	storeFD, err := unix.Open(spec.StorePhys, unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer unix.Close(storeFD)
	sentinel, err := os.OpenFile(spec.Sentinel, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer sentinel.Close()

	root, err := os.MkdirTemp("", "goonix-sandbox-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	for _, d := range []string{"bin", "dev", "nix/store", "tmp", "proc", "old", "build"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}
	// The self-bind has to come before the other mounts. pivot_root keeps
	// mounts that sit on that bind; mounts made earlier are covered by it.
	if err := unix.Mount(root, root, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	if err := unix.Mount("", root, "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
		return err
	}
	shReal, err := filepath.EvalSymlinks("/bin/sh")
	if err != nil {
		return err
	}
	if err := bindFileRO(shReal, filepath.Join(root, "bin/sh")); err != nil {
		return err
	}
	if err := bindDynamicLibs(shReal, root); err != nil {
		return err
	}
	for _, name := range []string{"null", "zero", "random", "urandom", "full"} {
		host := "/dev/" + name
		if _, err := os.Lstat(host); err != nil {
			continue
		}
		dst := filepath.Join(root, "dev", name)
		if err := os.WriteFile(dst, nil, 0o666); err != nil {
			return err
		}
		if err := unix.Mount(host, dst, "", unix.MS_BIND, ""); err != nil {
			return err
		}
	}
	if err := unix.Mount("proc", filepath.Join(root, "proc"), "proc", unix.MS_NOSUID|unix.MS_NODEV|unix.MS_NOEXEC, ""); err != nil {
		return err
	}
	if err := unix.Mount("tmpfs", filepath.Join(root, "nix/store"), "tmpfs", 0, "mode=1777"); err != nil {
		return err
	}
	for _, phys := range spec.Closure {
		if err := mountClosurePath(root, phys); err != nil {
			return err
		}
	}
	if spec.BuildDir == "" || !filepath.IsAbs(spec.BuildDir) {
		return fmt.Errorf("build directory %q is not absolute", spec.BuildDir)
	}
	// Nix builds with cwd /build. The host temp directory is bind-mounted there.
	buildDest := filepath.Join(root, "build")
	if err := unix.Mount(spec.BuildDir, buildDest, "", unix.MS_BIND, ""); err != nil {
		return err
	}

	if err := unix.Chdir(root); err != nil {
		return err
	}
	if err := unix.PivotRoot(".", "old"); err != nil {
		return err
	}
	if err := unix.Chdir("/"); err != nil {
		return err
	}
	if err := unix.Unmount("/old", unix.MNT_DETACH); err != nil {
		return err
	}
	_ = os.Remove("/old")
	if _, err := sentinel.WriteString("ok\n"); err != nil {
		return err
	}

	attr := &syscall.ProcAttr{
		Dir:   "/build",
		Env:   spec.Env,
		Files: []uintptr{os.Stdin.Fd(), os.Stdout.Fd(), os.Stderr.Fd()},
		Sys: &syscall.SysProcAttr{
			Cloneflags: unix.CLONE_NEWUSER,
			UidMappings: []syscall.SysProcIDMap{{
				ContainerID: 1000,
				HostID:      0,
				Size:        1,
			}},
			GidMappings: []syscall.SysProcIDMap{{
				ContainerID: 1000,
				HostID:      0,
				Size:        1,
			}},
			Credential: &syscall.Credential{Uid: 1000, Gid: 1000},
		},
	}
	pid, err := syscall.ForkExec(spec.Builder, spec.Args, attr)
	if err != nil {
		return err
	}
	var ws syscall.WaitStatus
	if _, err := syscall.Wait4(pid, &ws, 0, nil); err != nil {
		return err
	}
	if !ws.Exited() || ws.ExitStatus() != 0 {
		code := 1
		if ws.Exited() {
			code = ws.ExitStatus()
		}
		return sandboxExit(code)
	}
	for _, phys := range spec.Outputs {
		if err := copyOutputToHost(storeFD, "/nix/store/"+filepath.Base(phys), filepath.Base(phys)); err != nil {
			return err
		}
	}
	return nil
}

func mountClosurePath(root, phys string) error {
	fi, err := os.Lstat(phys)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, "nix/store", filepath.Base(phys))
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(phys)
		if err != nil {
			return err
		}
		return os.Symlink(target, dest)
	}
	if fi.IsDir() {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(dest, os.O_CREATE, 0o644)
		if err != nil {
			return err
		}
		f.Close()
	}
	if err := unix.Mount(phys, dest, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	return unix.Mount("", dest, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, "")
}

func bindFileRO(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(dst, os.O_CREATE, 0o755)
	if err != nil {
		return err
	}
	f.Close()
	if err := unix.Mount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return err
	}
	return unix.Mount("", dst, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, "")
}

func bindDynamicLibs(bin, root string) error {
	ldd, err := exec.LookPath("ldd")
	if err != nil {
		return err
	}
	out, err := exec.Command(ldd, bin).Output()
	if err != nil {
		return err
	}
	for _, line := range strings.Split(string(out), "\n") {
		for _, field := range strings.Fields(line) {
			if !strings.HasPrefix(field, "/") {
				continue
			}
			real, err := filepath.EvalSymlinks(field)
			if err != nil {
				real = field
			}
			if err := bindFileRO(real, root+real); err != nil {
				return err
			}
			if real == field {
				continue
			}
			link := root + field
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				return err
			}
			rel, err := filepath.Rel(filepath.Dir(field), real)
			if err != nil {
				return err
			}
			_ = os.Remove(link)
			if err := os.Symlink(rel, link); err != nil && !os.IsExist(err) {
				return err
			}
		}
	}
	return nil
}

func copyOutputToHost(storeFD int, src, name string) error {
	fi, err := os.Lstat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return copyAt(storeFD, name, src, fi)
}

func copyAt(dirfd int, name, src string, fi os.FileInfo) error {
	if fi.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return unix.Symlinkat(target, dirfd, name)
	}
	if fi.IsDir() {
		if err := unix.Mkdirat(dirfd, name, uint32(fi.Mode().Perm())); err != nil && !os.IsExist(err) {
			return err
		}
		child, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_DIRECTORY, 0)
		if err != nil {
			return err
		}
		defer unix.Close(child)
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			cfi, err := os.Lstat(filepath.Join(src, e.Name()))
			if err != nil {
				return err
			}
			if err := copyAt(child, e.Name(), filepath.Join(src, e.Name()), cfi); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fd, err := unix.Openat(dirfd, name, unix.O_CREAT|unix.O_WRONLY|unix.O_TRUNC, uint32(fi.Mode().Perm()))
	if err != nil {
		return err
	}
	defer unix.Close(fd)
	buf := make([]byte, 32*1024)
	for {
		n, rerr := in.Read(buf)
		if n > 0 {
			if err := writeFull(fd, buf[:n]); err != nil {
				return err
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

func writeFull(fd int, p []byte) error {
	for len(p) > 0 {
		n, err := unix.Write(fd, p)
		if n > 0 {
			p = p[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
