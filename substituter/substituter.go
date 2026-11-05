// Copyright 2026 The goonix authors
//
// SPDX-License-Identifier: Apache-2.0

// Package substituter fetches signed paths from a Nix binary cache (Phase 6).
package substituter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/nix-community/go-nix/pkg/narinfo"
	"github.com/nix-community/go-nix/pkg/narinfo/signature"
	"github.com/nix-community/go-nix/pkg/nixbase32"
	"github.com/nix-community/go-nix/pkg/nixhash"
	"github.com/nix-community/go-nix/pkg/storepath"

	"github.com/connerohnesorge/goonix/store/local"
)

const defaultCache = "https://cache.nixos.org"

// Config configures binary cache access.
type Config struct {
	CacheURL   string
	HTTPClient *http.Client
	// TrustedKeys defaults to cache.nixos.org-1 when empty.
	TrustedKeys []signature.PublicKey
	// RequireSig rejects substitution when no valid signature is present (default true).
	RequireSig *bool
}

// Client downloads narinfo + NAR payloads into a local store.
type Client struct {
	cfg Config
}

// NewClient constructs a substituter client.
func NewClient(cfg Config) *Client {
	if cfg.CacheURL == "" {
		cfg.CacheURL = defaultCache
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Minute}
	}
	if len(cfg.TrustedKeys) == 0 {
		pk, err := signature.ParsePublicKey(cacheNixOSPublicKey)
		if err == nil {
			cfg.TrustedKeys = []signature.PublicKey{pk}
		}
	}
	return &Client{cfg: cfg}
}

// cache.nixos.org-1 trusted public key (same as Nix defaults).
const cacheNixOSPublicKey = "cache.nixos.org-1:6NCHdD59X431o0gWypbMrAURkbJ16ZPMQFGspcDShjY="

// TrySubstitute fetches storePath from the cache when possible.
func (c *Client) TrySubstitute(st *local.Store, storePath string) (bool, error) {
	if c == nil {
		return false, nil
	}
	ok, err := st.IsValidPath(storePath)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	ni, err := c.FetchNarInfo(storePath)
	if err != nil {
		if isNetworkUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	if err := c.verifyNarInfo(ni); err != nil {
		return false, err
	}
	for _, ref := range ni.References {
		refPath := ref
		if !strings.HasPrefix(refPath, storepath.StoreDir+"/") {
			refPath = storepath.StoreDir + "/" + ref
		}
		refOK, err := c.TrySubstitute(st, refPath)
		if err != nil {
			return false, err
		}
		if !refOK {
			return false, fmt.Errorf("reference %s not substituted", refPath)
		}
	}
	data, err := c.downloadNar(ni)
	if err != nil {
		if isNetworkUnavailable(err) {
			return false, nil
		}
		return false, err
	}
	narHash := ni.NarHash.Format(nixhash.NixBase32, false)
	if err := st.ImportNAR(storePath, bytes.NewReader(data), narHash); err != nil {
		return false, err
	}
	return true, nil
}

func isNetworkUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") ||
		strings.Contains(msg, "i/o timeout")
}

func (c *Client) requireSig() bool {
	if c.cfg.RequireSig == nil {
		return true
	}
	return *c.cfg.RequireSig
}

// FetchNarInfo downloads and parses .narinfo for storePath.
func (c *Client) FetchNarInfo(storePath string) (*narinfo.NarInfo, error) {
	u, err := c.narInfoURL(storePath)
	if err != nil {
		return nil, err
	}
	resp, err := c.cfg.HTTPClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("narinfo HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	ni, err := narinfo.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	ni.StorePath = storePath
	for i, ref := range ni.References {
		if strings.HasPrefix(ref, storepath.StoreDir+"/") {
			continue
		}
		ni.References[i] = ref
	}
	return ni, nil
}

// NarInfoURL returns the binary-cache .narinfo URL for storePath (hash-only basename).
func (c *Client) NarInfoURL(storePath string) (string, error) {
	return c.narInfoURL(storePath)
}

func (c *Client) narInfoURL(storePath string) (string, error) {
	sp, err := storepath.FromAbsolutePath(storePath)
	if err != nil {
		return "", err
	}
	hashName := nixbase32.EncodeToString(sp.Digest) + ".narinfo"
	base, err := url.Parse(c.cfg.CacheURL)
	if err != nil {
		return "", err
	}
	base.Path = path.Join(base.Path, hashName)
	return base.String(), nil
}

func (c *Client) verifyNarInfo(ni *narinfo.NarInfo) error {
	if err := ni.Check(); err != nil {
		return err
	}
	if len(c.cfg.TrustedKeys) == 0 {
		return nil
	}
	if len(ni.Signatures) == 0 {
		if c.requireSig() {
			return fmt.Errorf("missing signature for %s", ni.StorePath)
		}
		return nil
	}
	fp := ni.Fingerprint()
	if !signature.VerifyFirst(fp, ni.Signatures, c.cfg.TrustedKeys) {
		return fmt.Errorf("no trusted signature for %s", ni.StorePath)
	}
	return nil
}

func (c *Client) downloadNar(ni *narinfo.NarInfo) ([]byte, error) {
	u, err := c.narURL(ni.URL)
	if err != nil {
		return nil, err
	}
	resp, err := c.cfg.HTTPClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nar HTTP %s", resp.Status)
	}
	compressed, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return decompress(compressed, ni.Compression)
}

func (c *Client) narURL(rel string) (string, error) {
	base, err := url.Parse(c.cfg.CacheURL)
	if err != nil {
		return "", err
	}
	base.Path = path.Join(base.Path, rel)
	return base.String(), nil
}
