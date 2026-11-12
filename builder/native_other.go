// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package builder

import "errors"

// nativeBuilds is false where builds run in the Linux sandbox.
const nativeBuilds = false

func spawnNative(*nativeSpawn) error {
	return errors.New("native builds run only on Windows")
}

func checkNativeSystem(string, string) error { return nil }
