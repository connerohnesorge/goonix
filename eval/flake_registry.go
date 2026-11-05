// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/connerohnesorge/goonix/parser"
)

// flakeInput is the subset of a Nix fetcher input that registry lookup uses.
// Attrs compare the way Nix Attrs equality does, including type.
type flakeInput struct {
	attrs map[string]string
}

func (in flakeInput) get(key string) string {
	if in.attrs == nil {
		return ""
	}
	return in.attrs[key]
}

func (in flakeInput) isDirect() bool {
	return in.get("type") != "" && in.get("type") != "indirect"
}

func (in flakeInput) toString() string {
	switch in.get("type") {
	case "indirect":
		s := "flake:" + in.get("id")
		if ref := in.get("ref"); ref != "" {
			s += "/" + ref
		}
		if rev := in.get("rev"); rev != "" {
			s += "/" + rev
		}
		return s
	case "path":
		return "path:" + in.get("path")
	case "github", "gitlab", "sourcehut":
		return flakeRefFromInput(in).String()
	default:
		return ""
	}
}

func (in flakeInput) equal(other flakeInput) bool {
	if len(in.attrs) != len(other.attrs) {
		return false
	}
	for k, v := range in.attrs {
		if other.attrs[k] != v {
			return false
		}
	}
	return true
}

// contains matches Nix Input::contains: equal, or equal after erasing ref and
// rev on the looked-up input.
func (in flakeInput) contains(other flakeInput) bool {
	if in.equal(other) {
		return true
	}
	stripped := flakeInput{attrs: map[string]string{}}
	for k, v := range other.attrs {
		if k == "ref" || k == "rev" {
			continue
		}
		stripped.attrs[k] = v
	}
	return in.equal(stripped)
}

func (in flakeInput) applyOverrides(ref, rev string) (flakeInput, error) {
	out := flakeInput{attrs: map[string]string{}}
	for k, v := range in.attrs {
		out.attrs[k] = v
	}
	switch in.get("type") {
	case "indirect":
		if rev != "" {
			out.attrs["rev"] = rev
		}
		if ref != "" {
			out.attrs["ref"] = ref
		}
		return out, nil
	case "github", "gitlab", "sourcehut":
		if rev != "" && ref != "" {
			return flakeInput{}, fmt.Errorf("cannot apply both a commit hash (%s) and a branch/tag name ('%s') to input '%s'", rev, ref, in.toString())
		}
		if rev != "" {
			out.attrs["rev"] = rev
			delete(out.attrs, "ref")
		}
		if ref != "" {
			out.attrs["ref"] = ref
			delete(out.attrs, "rev")
		}
		return out, nil
	default:
		if ref != "" {
			return flakeInput{}, fmt.Errorf("don't know how to set branch/tag name of input '%s' to '%s'", in.toString(), ref)
		}
		if rev != "" {
			return flakeInput{}, fmt.Errorf("don't know how to set revision of input '%s' to '%s'", in.toString(), rev)
		}
		return out, nil
	}
}

type registryEntry struct {
	from  flakeInput
	to    flakeInput
	exact bool
}

func flakeInputFromRef(f flakeRef) flakeInput {
	attrs := map[string]string{"type": f.Type}
	switch f.Type {
	case "indirect":
		attrs["id"] = f.ID
		if f.Ref != "" {
			attrs["ref"] = f.Ref
		}
		if f.Rev != "" {
			attrs["rev"] = f.Rev
		}
	case "path":
		attrs["path"] = f.Path
	case "github", "gitlab", "sourcehut":
		attrs["owner"] = f.Owner
		attrs["repo"] = f.Repo
		if f.Ref != "" {
			attrs["ref"] = f.Ref
		}
		if f.Rev != "" {
			attrs["rev"] = f.Rev
		}
		if f.Host != "" {
			attrs["host"] = f.Host
		}
	}
	if f.NarHash != "" {
		attrs["narHash"] = f.NarHash
	}
	return flakeInput{attrs: attrs}
}

func flakeRefFromInput(in flakeInput) flakeRef {
	return flakeRef{
		Type:    in.get("type"),
		ID:      in.get("id"),
		Owner:   in.get("owner"),
		Repo:    in.get("repo"),
		Rev:     in.get("rev"),
		Ref:     in.get("ref"),
		Host:    in.get("host"),
		NarHash: in.get("narHash"),
		Path:    in.get("path"),
		Dir:     in.get("dir"),
	}
}

func (ev *Evaluator) fetchResolvedTree(pos parser.Pos, in flakeInput) (*Value, error) {
	switch in.get("type") {
	case "path":
		p := in.get("path")
		if (strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../")) && ev.curFile != "" {
			p = filepath.Join(filepath.Dir(ev.curFile), p)
		}
		return ev.treeFromDir(pos, p, "source", treeMeta{}, true)
	case "github":
		rev := in.get("rev")
		if rev == "" {
			rev = in.get("ref")
		}
		return ev.fetchTreeGitHub(in.get("owner"), in.get("repo"), rev)
	case "gitlab":
		rev := in.get("rev")
		if rev == "" {
			rev = in.get("ref")
		}
		return ev.fetchTreeGitLab(in.get("owner"), in.get("repo"), rev)
	case "sourcehut":
		rev := in.get("rev")
		if rev == "" {
			rev = in.get("ref")
		}
		return ev.fetchTreeSourceHut(in.get("owner"), in.get("repo"), rev)
	default:
		return nil, evalErr(pos, "cannot show unsupported input '{\"type\":%q}'", in.get("type"))
	}
}

func lookupFlakeInput(input flakeInput) (flakeInput, error) {
	regs, err := loadFlakeRegistries()
	if err != nil {
		return flakeInput{}, err
	}
	for n := 1; ; n++ {
		if n > 100 {
			return flakeInput{}, fmt.Errorf("cycle detected in flake registry for '%s'", input.toString())
		}
		matched := false
		for _, entries := range regs {
			for _, entry := range entries {
				if entry.exact {
					if entry.from.equal(input) {
						input = entry.to
						matched = true
						break
					}
					continue
				}
				if entry.from.contains(input) {
					ref, rev := "", ""
					if entry.from.get("ref") == "" {
						ref = input.get("ref")
					}
					if entry.from.get("rev") == "" {
						rev = input.get("rev")
					}
					next, err := entry.to.applyOverrides(ref, rev)
					if err != nil {
						return flakeInput{}, err
					}
					input = next
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			break
		}
	}
	if !input.isDirect() {
		return flakeInput{}, fmt.Errorf("cannot find flake '%s' in the flake registries", input.toString())
	}
	return input, nil
}

func loadFlakeRegistries() ([][]registryEntry, error) {
	user, err := readRegistryFile(userRegistryPath())
	if err != nil {
		return nil, err
	}
	system, err := readRegistryFile(systemRegistryPath())
	if err != nil {
		return nil, err
	}
	global, err := readGlobalRegistry()
	if err != nil {
		return nil, err
	}
	// Flag registry is empty unless --override-flake is passed. Order is
	// flag, user, system, global.
	return [][]registryEntry{nil, user, system, global}, nil
}

func userRegistryPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nix", "registry.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "/"
	}
	return filepath.Join(home, ".config", "nix", "registry.json")
}

func systemRegistryPath() string {
	dir := os.Getenv("NIX_CONF_DIR")
	if dir == "" {
		dir = "/etc/nix"
	}
	if i := strings.IndexByte(dir, ':'); i >= 0 {
		dir = dir[:i]
	}
	return filepath.Join(dir, "registry.json")
}

func readGlobalRegistry() ([]registryEntry, error) {
	setting := flakeRegistrySetting()
	if setting == "" {
		return nil, nil
	}
	if strings.HasPrefix(setting, "/") {
		return readRegistryFile(setting)
	}
	body, err := cachedRegistryDownload(setting)
	if err != nil {
		return nil, err
	}
	return parseRegistryBytes(setting, body)
}

func flakeRegistrySetting() string {
	if v, ok := nixSetting("flake-registry"); ok {
		return v
	}
	return "https://channels.nixos.org/flake-registry.json"
}

func nixSetting(key string) (string, bool) {
	if v, ok := parseNixConf(os.Getenv("NIX_CONFIG"))[key]; ok {
		return v, true
	}
	dirs := os.Getenv("NIX_CONF_DIR")
	if dirs != "" {
		for _, dir := range strings.Split(dirs, ":") {
			if dir == "" {
				continue
			}
			if v, ok := parseNixConfFile(filepath.Join(dir, "nix.conf"))[key]; ok {
				return v, true
			}
		}
		return "", false
	}
	if v, ok := parseNixConfFile(userNixConfPath())[key]; ok {
		return v, true
	}
	if v, ok := parseNixConfFile("/etc/nix/nix.conf")[key]; ok {
		return v, true
	}
	return "", false
}

func userNixConfPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "nix", "nix.conf")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "nix", "nix.conf")
}

func parseNixConfFile(path string) map[string]string {
	if path == "" {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseNixConf(string(body))
}

func parseNixConf(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return out
}

func readRegistryFile(path string) ([]registryEntry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		fmt.Fprintf(os.Stderr, "warning: cannot read flake registry '%s': %s\n", path, err.Error())
		return nil, nil
	}
	return parseRegistryBytes(path, body)
}

func parseRegistryBytes(path string, body []byte) ([]registryEntry, error) {
	var doc struct {
		Version int `json:"version"`
		Flakes  []struct {
			From  map[string]any `json:"from"`
			To    map[string]any `json:"to"`
			Exact bool           `json:"exact"`
		} `json:"flakes"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot parse flake registry '%s': %s\n", path, err.Error())
		return nil, nil
	}
	if doc.Version != 2 {
		fmt.Fprintf(os.Stderr, "warning: cannot read flake registry '%s': flake registry '%s' has unsupported version %d\n", path, path, doc.Version) // stub-scan: exempt
		return nil, nil
	}
	out := make([]registryEntry, 0, len(doc.Flakes))
	for _, flake := range doc.Flakes {
		to := attrsFromJSON(flake.To)
		delete(to.attrs, "dir")
		out = append(out, registryEntry{
			from:  attrsFromJSON(flake.From),
			to:    to,
			exact: flake.Exact,
		})
	}
	return out, nil
}

func attrsFromJSON(raw map[string]any) flakeInput {
	attrs := map[string]string{}
	for k, v := range raw {
		s, ok := v.(string)
		if !ok {
			continue
		}
		attrs[k] = s
	}
	return flakeInput{attrs: attrs}
}

var (
	registryDownloadMu sync.Mutex
	registryDownload   = map[string][]byte{}
)

func cachedRegistryDownload(rawURL string) ([]byte, error) {
	registryDownloadMu.Lock()
	if body, ok := registryDownload[rawURL]; ok {
		registryDownloadMu.Unlock()
		return body, nil
	}
	registryDownloadMu.Unlock()
	body, err := downloadChecked(rawURL)
	if err != nil {
		return nil, err
	}
	registryDownloadMu.Lock()
	registryDownload[rawURL] = body
	registryDownloadMu.Unlock()
	return body, nil
}
