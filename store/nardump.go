// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	gonixnar "github.com/nix-community/go-nix/pkg/nar"
)

// dumpNAR writes the NAR of the filesystem object at p to w. It follows Nix's
// dump: entries sorted by name, symlinks not followed. The executable flag,
// link-target spelling and entry names come from the platform layer, so a
// Windows store reads the exec stream, '/' link targets and case-hack names.
func dumpNAR(w io.Writer, p string, filter func(string, NodeType) bool) error {
	nw, err := gonixnar.NewWriter(w)
	if err != nil {
		return err
	}
	defer nw.Close()
	if err := dumpNARNode(nw, p, "/", filter); err != nil {
		return err
	}
	return nw.Close()
}

func dumpNARNode(nw *gonixnar.Writer, p, sub string, filter func(string, NodeType) bool) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	var t NodeType
	switch {
	case fi.Mode()&os.ModeSymlink != 0:
		t = NodeTypeSymlink
	case fi.IsDir():
		t = NodeTypeDir
	case fi.Mode().IsRegular():
		t = NodeTypeRegular
	default:
		return fmt.Errorf("file '%s' has an unsupported type", p)
	}
	if filter != nil && !filter(p, t) {
		return nil
	}
	switch t {
	case NodeTypeSymlink:
		target, err := ReadLinkTarget(p)
		if err != nil {
			return err
		}
		return nw.WriteHeader(&gonixnar.Header{Path: sub, Type: gonixnar.TypeSymlink, LinkTarget: target})
	case NodeTypeDir:
		if err := nw.WriteHeader(&gonixnar.Header{Path: sub, Type: gonixnar.TypeDirectory}); err != nil {
			return err
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return err
		}
		type child struct{ nar, disk string }
		children := make([]child, 0, len(entries))
		for _, e := range entries {
			children = append(children, child{nar: narEntryName(e.Name()), disk: e.Name()})
		}
		sort.Slice(children, func(i, j int) bool { return children[i].nar < children[j].nar })
		for _, c := range children {
			if err := dumpNARNode(nw, filepath.Join(p, c.disk), path.Join(sub, c.nar), filter); err != nil {
				return err
			}
		}
		return nil
	default:
		if err := nw.WriteHeader(&gonixnar.Header{
			Path:       sub,
			Type:       gonixnar.TypeRegular,
			Size:       fi.Size(),
			Executable: IsExecutable(p, fi),
		}); err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		n, err := io.Copy(nw, f)
		if err != nil {
			return err
		}
		if n != fi.Size() {
			return fmt.Errorf("file '%s' changed size while it was being read", p)
		}
		return nil
	}
}
