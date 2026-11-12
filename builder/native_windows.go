// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package builder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// nativeBuilds is true on Windows: there is no sandbox, and the builder runs
// directly on the host against the physical store.
const nativeBuilds = true

// nativeSystems are the systems a Windows host builds natively: its own, and
// i686 on x86_64 through WoW64, as NovaNix allows.
func nativeSystems() []string {
	switch runtime.GOARCH {
	case "amd64":
		return []string{"x86_64-windows", "i686-windows"}
	case "arm64":
		return []string{"aarch64-windows"}
	case "386":
		return []string{"i686-windows"}
	}
	return nil
}

func checkNativeSystem(drvPath, system string) error {
	systems := nativeSystems()
	for _, s := range systems {
		if s == system {
			return nil
		}
	}
	host := ""
	if len(systems) > 0 {
		host = systems[0]
	}
	return fmt.Errorf("a '%s' with features {} is required to build '%s', but I am a '%s' with features {}", system, drvPath, host)
}

var procNtResumeProcess = windows.NewLazySystemDLL("ntdll.dll").NewProc("NtResumeProcess")

// spawnNative runs the builder in a job object that kills every process the
// build started when the build ends. The process starts suspended and joins
// the job before it runs, so nothing it spawns escapes the job.
func spawnNative(sp *nativeSpawn) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("creating job object: %w", err)
	}
	defer windows.CloseHandle(job)
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return fmt.Errorf("configuring job object: %w", err)
	}

	cmd := exec.Command(sp.Builder, sp.Args...)
	attr := &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED}
	// NovaNix runs `cmd.exe /c ...` through the shell: the arguments after
	// /c are joined with spaces and handed to cmd as one command line.
	if strings.EqualFold(filepath.Base(sp.Builder), "cmd.exe") && len(sp.Args) > 0 && strings.EqualFold(sp.Args[0], "/c") {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = sp.Builder
		}
		cmd.Path = comspec
		attr.CmdLine = `"` + comspec + `" /c ` + strings.Join(sp.Args[1:], " ")
	}
	cmd.SysProcAttr = attr
	cmd.Dir = sp.BuildDir
	cmd.Env = sp.Env
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("cannot spawn builder '%s': %w", sp.Builder, err)
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_SUSPEND_RESUME, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, h)
		if err == nil {
			if r, _, _ := procNtResumeProcess.Call(uintptr(h)); r != 0 {
				err = fmt.Errorf("NtResumeProcess: status 0x%x", r)
			}
		}
		windows.CloseHandle(h)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("starting builder in a job object: %w", err)
	}
	werr := cmd.Wait()
	// Anything the builder left running dies with the build.
	_ = windows.TerminateJobObject(job, 1)
	if werr != nil {
		if ee, ok := werr.(*exec.ExitError); ok {
			return fmt.Errorf("builder failed with exit code %d", ee.ExitCode())
		}
		return werr
	}
	return nil
}
