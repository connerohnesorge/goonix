// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

package builder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gonixdrv "github.com/nix-community/go-nix/pkg/derivation"

	"github.com/connerohnesorge/goonix/store"
)

var shVarName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// writeStructuredAttrsFiles writes .attrs.json and .attrs.sh the way Nix 2.24.9
// prepareStructuredAttrs + writeStructuredAttrsShell do. The derivation env
// keeps only __json; these files are what the builder reads.
func (b *Builder) writeStructuredAttrsFiles(d *gonixdrv.Derivation, outPaths map[string]string, tmpDir string) error {
	if d == nil || d.Env == nil || d.Env["__json"] == "" {
		return nil
	}
	body, shell, err := b.structuredAttrsBodies(d, outPaths, nil)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".attrs.json"), []byte(body), 0o444); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(tmpDir, ".attrs.sh"), []byte(shell), 0o444)
}

// structuredAttrsBodies renders .attrs.json and .attrs.sh. rewrite, when
// set, is applied to every string in the document before it is encoded (a
// native build's physical store spelling); output placeholders map to
// outPaths, or to rewrite's result for them.
func (b *Builder) structuredAttrsBodies(d *gonixdrv.Derivation, outPaths map[string]string, rewrite func(string) string) (string, string, error) {
	doc, err := parseNixJSON(d.Env["__json"])
	if err != nil {
		return "", "", fmt.Errorf("cannot process __json attribute: %s", err.Error())
	}
	if err := b.expandExportReferencesGraph(d, doc); err != nil {
		return "", "", err
	}
	outputs := map[string]any{}
	names := make([]string, 0, len(outPaths))
	for name := range outPaths {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		outputs[name] = store.OutputPlaceholder(name)
	}
	doc["outputs"] = outputs
	if rewrite != nil {
		doc = rewriteJSONStrings(doc, rewrite).(map[string]any)
		body, err := marshalNixJSON(doc)
		if err != nil {
			return "", "", err
		}
		return string(body), writeStructuredAttrsShell(doc), nil
	}

	body, err := marshalNixJSON(doc)
	if err != nil {
		return "", "", err
	}
	for _, name := range names {
		body = bytes.ReplaceAll(body, []byte(store.OutputPlaceholder(name)), []byte(outPaths[name]))
	}
	rewritten, err := parseNixJSON(string(body))
	if err != nil {
		return "", "", err
	}
	return string(body), writeStructuredAttrsShell(rewritten), nil
}

// rewriteJSONStrings applies f to every string in a decoded JSON value.
func rewriteJSONStrings(v any, f func(string) string) any {
	switch t := v.(type) {
	case string:
		return f(t)
	case map[string]any:
		for k, e := range t {
			t[k] = rewriteJSONStrings(e, f)
		}
		return t
	case []any:
		for i, e := range t {
			t[i] = rewriteJSONStrings(e, f)
		}
		return t
	default:
		return v
	}
}

func (b *Builder) structuredAttrsEnv(d *gonixdrv.Derivation, tmpDir string) []string {
	if d == nil || d.Env["__json"] == "" {
		return nil
	}
	return []string{
		"NIX_ATTRS_JSON_FILE=" + filepath.Join(tmpDir, ".attrs.json"),
		"NIX_ATTRS_SH_FILE=" + filepath.Join(tmpDir, ".attrs.sh"),
	}
}

func parseNixJSON(raw string) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func marshalNixJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func writeStructuredAttrsShell(doc map[string]any) string {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, key := range keys {
		if !shVarName.MatchString(key) {
			continue
		}
		value := doc[key]
		if s, ok := handleSimpleType(value); ok {
			fmt.Fprintf(&b, "declare %s=%s\n", key, s)
			continue
		}
		switch typed := value.(type) {
		case []any:
			parts := make([]string, 0, len(typed))
			good := true
			for _, el := range typed {
				s, ok := handleSimpleType(el)
				if !ok {
					good = false
					break
				}
				parts = append(parts, s)
			}
			if good {
				fmt.Fprintf(&b, "declare -a %s=(%s)\n", key, strings.Join(append(parts, ""), " "))
			}
		case map[string]any:
			sub := make([]string, 0, len(typed))
			for k := range typed {
				sub = append(sub, k)
			}
			sort.Strings(sub)
			good := true
			var body strings.Builder
			for _, k := range sub {
				s, ok := handleSimpleType(typed[k])
				if !ok {
					good = false
					break
				}
				fmt.Fprintf(&body, "[%s]=%s ", shellEscape(k), s)
			}
			if good {
				fmt.Fprintf(&b, "declare -A %s=(%s)\n", key, body.String())
			}
		}
	}
	return b.String()
}

func handleSimpleType(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return shellEscape(v), true
	case json.Number:
		if isJSONInt(v.String()) {
			return v.String(), true
		}
		f, err := v.Float64()
		if err != nil || f != float64(int64(f)) {
			return "", false
		}
		return fmt.Sprintf("%d", int64(f)), true
	case nil:
		return "''", true
	case bool:
		if v {
			return "1", true
		}
		return "", true
	default:
		return "", false
	}
}

func isJSONInt(s string) bool {
	if s == "" || s == "-" {
		return false
	}
	i := 0
	if s[0] == '-' {
		i = 1
	}
	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func (b *Builder) expandExportReferencesGraph(d *gonixdrv.Derivation, doc map[string]any) error {
	raw, ok := doc["exportReferencesGraph"]
	if !ok {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	inputs, err := b.logicalInputClosure(d)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		arr, ok := obj[key].([]any)
		if !ok {
			return fmt.Errorf("exportReferencesGraph.%s must be a list of store paths", key)
		}
		var roots []string
		for _, el := range arr {
			p, ok := el.(string)
			if !ok {
				return fmt.Errorf("exportReferencesGraph.%s must be a list of store paths", key)
			}
			if _, ok := inputs[p]; !ok {
				return fmt.Errorf("cannot export references of path '%s' because it is not in the input closure of the derivation", p)
			}
			roots = append(roots, p)
		}
		closure, err := b.exportReferenceClosure(roots)
		if err != nil {
			return err
		}
		infos := make([]any, 0, len(closure))
		for _, p := range closure {
			info, err := b.pathInfoJSON(p)
			if err != nil {
				return err
			}
			infos = append(infos, info)
		}
		doc[key] = infos
	}
	return nil
}

func (b *Builder) logicalInputClosure(d *gonixdrv.Derivation) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	var walk func(string) error
	walk = func(p string) error {
		if p == "" {
			return nil
		}
		if _, ok := set[p]; ok {
			return nil
		}
		ok, err := b.Store.IsValidPath(p)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		set[p] = struct{}{}
		refs, err := b.Store.QueryReferences(p)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := walk(ref); err != nil {
				return err
			}
		}
		return nil
	}
	for _, src := range d.InputSources {
		if err := walk(src); err != nil {
			return nil, err
		}
	}
	if b.Load == nil {
		return set, nil
	}
	for drvPath, outs := range d.InputDerivations {
		dep, err := b.Load(drvPath)
		if err != nil || dep == nil {
			continue
		}
		depOuts, err := store.LogicalOutputPaths(dep)
		if err != nil {
			return nil, err
		}
		for _, name := range outs {
			if err := walk(depOuts[name]); err != nil {
				return nil, err
			}
		}
	}
	return set, nil
}

func (b *Builder) exportReferenceClosure(roots []string) ([]string, error) {
	seen := map[string]struct{}{}
	var walk func(string) error
	walk = func(p string) error {
		if _, ok := seen[p]; ok {
			return nil
		}
		seen[p] = struct{}{}
		refs, err := b.Store.QueryReferences(p)
		if err != nil {
			return err
		}
		for _, ref := range refs {
			if err := walk(ref); err != nil {
				return err
			}
		}
		if strings.HasSuffix(p, ".drv") && b.Load != nil {
			dep, err := b.Load(p)
			if err != nil || dep == nil {
				return nil
			}
			depOuts, err := store.LogicalOutputPaths(dep)
			if err != nil {
				return err
			}
			for _, out := range depOuts {
				if err := walk(out); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, root := range roots {
		if err := walk(root); err != nil {
			return nil, err
		}
	}
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func (b *Builder) pathInfoJSON(logical string) (map[string]any, error) {
	meta, err := b.Store.QueryPathMeta(logical)
	if err != nil {
		return nil, err
	}
	closure, err := b.exportReferenceClosure([]string{logical})
	if err != nil {
		return nil, err
	}
	var closureSize int64
	for _, p := range closure {
		info, err := b.Store.QueryPathMeta(p)
		if err != nil {
			return nil, err
		}
		closureSize += info.NarSize
	}
	hash := meta.NarHash
	if !strings.Contains(hash, ":") {
		hash = "sha256:" + hash
	}
	refs := meta.Refs
	if refs == nil {
		refs = []string{}
	}
	return map[string]any{
		"closureSize": closureSize,
		"narHash":     hash,
		"narSize":     meta.NarSize,
		"path":        logical,
		"references":  refs,
		"valid":       true,
	}, nil
}

type builtOutput struct {
	refs    []string
	narSize int64
}

func (b *Builder) finishStructuredOutputs(drvPath string, d *gonixdrv.Derivation, outPaths map[string]string, infos map[string]builtOutput) error {
	if d == nil || d.Env["__json"] == "" {
		return nil
	}
	doc, err := parseNixJSON(d.Env["__json"])
	if err != nil {
		return fmt.Errorf("cannot process __json attribute of '%s': %s", drvPath, err.Error())
	}
	checks, _ := doc["outputChecks"].(map[string]any)
	discard, _ := doc["unsafeDiscardReferences"].(map[string]any)
	for name, info := range infos {
		if discard != nil {
			if flag, ok := discard[name].(bool); ok && flag {
				infos[name] = builtOutput{refs: nil, narSize: info.narSize}
				info.refs = nil
			}
		}
		spec, _ := checks[name].(map[string]any)
		if spec == nil {
			continue
		}
		if err := b.applyOutputChecks(name, outPaths[name], info, spec, outPaths, infos); err != nil {
			return err
		}
	}
	return nil
}

func (b *Builder) applyOutputChecks(name, logical string, info builtOutput, spec map[string]any, outPaths map[string]string, infos map[string]builtOutput) error {
	if raw, ok := spec["maxSize"]; ok {
		limit, err := jsonUint(raw)
		if err != nil {
			return fmt.Errorf("attribute 'outputChecks.%s.maxSize' must be an integer", name)
		}
		if info.narSize > limit {
			return fmt.Errorf("path '%s' is too large at %d bytes; limit is %d bytes", logical, info.narSize, limit)
		}
	}
	if raw, ok := spec["maxClosureSize"]; ok {
		limit, err := jsonUint(raw)
		if err != nil {
			return fmt.Errorf("attribute 'outputChecks.%s.maxClosureSize' must be an integer", name)
		}
		_, size, err := b.outputClosure(logical, outPaths, infos)
		if err != nil {
			return err
		}
		if size > limit {
			return fmt.Errorf("closure of path '%s' is too large at %d bytes; limit is %d bytes", logical, size, limit)
		}
	}
	check := func(key string, allowed, recursive bool) error {
		raw, ok := spec[key]
		if !ok {
			return nil
		}
		arr, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("attribute '%s' of derivation '%s' must be a list of strings", key, logical)
		}
		specSet := map[string]struct{}{}
		for _, el := range arr {
			s, ok := el.(string)
			if !ok {
				return fmt.Errorf("attribute '%s' of derivation '%s' must be a list of strings", key, logical)
			}
			if isStorePath(s) {
				specSet[s] = struct{}{}
				continue
			}
			if p, ok := outPaths[s]; ok {
				specSet[p] = struct{}{}
				continue
			}
			return fmt.Errorf("derivation contains an illegal reference specifier '%s'", s)
		}
		var used map[string]struct{}
		if recursive {
			var err error
			used, _, err = b.outputClosure(logical, outPaths, infos)
			if err != nil {
				return err
			}
		} else {
			used = map[string]struct{}{}
			for _, r := range info.refs {
				used[r] = struct{}{}
			}
		}
		var bad []string
		for p := range used {
			_, in := specSet[p]
			if allowed && !in || !allowed && in {
				bad = append(bad, p)
			}
		}
		sort.Strings(bad)
		if len(bad) == 0 {
			return nil
		}
		var msg strings.Builder
		for _, p := range bad {
			msg.WriteString("\n  ")
			msg.WriteString(p)
		}
		return fmt.Errorf("output '%s' is not allowed to refer to the following paths:%s", logical, msg.String())
	}
	if err := check("allowedReferences", true, false); err != nil {
		return err
	}
	if err := check("allowedRequisites", true, true); err != nil {
		return err
	}
	if err := check("disallowedReferences", false, false); err != nil {
		return err
	}
	return check("disallowedRequisites", false, true)
}

func (b *Builder) outputClosure(start string, outPaths map[string]string, infos map[string]builtOutput) (map[string]struct{}, int64, error) {
	byPath := map[string]string{}
	for name, p := range outPaths {
		byPath[p] = name
	}
	seen := map[string]struct{}{}
	var size int64
	var walk func(string) error
	walk = func(p string) error {
		if _, ok := seen[p]; ok {
			return nil
		}
		seen[p] = struct{}{}
		if name, ok := byPath[p]; ok {
			info := infos[name]
			size += info.narSize
			for _, ref := range info.refs {
				if err := walk(ref); err != nil {
					return err
				}
			}
			return nil
		}
		meta, err := b.Store.QueryPathMeta(p)
		if err != nil {
			return err
		}
		size += meta.NarSize
		for _, ref := range meta.Refs {
			if err := walk(ref); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(start); err != nil {
		return nil, 0, err
	}
	return seen, size, nil
}

func jsonUint(v any) (int64, error) {
	switch n := v.(type) {
	case json.Number:
		return n.Int64()
	case float64:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("not an integer")
	}
}

func isStorePath(p string) bool {
	rest, ok := strings.CutPrefix(p, store.DefaultStoreDir+"/")
	return ok && rest != "" && !strings.Contains(rest, "/") && strings.Contains(rest, "-")
}
