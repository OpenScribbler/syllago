package capmon

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// ValidateSourceURL enforces the SSRF allowlist: HTTPS only, no raw IPs,
// no hostnames that resolve to reserved/private address space.
// Must be called for every source URL at pipeline startup — NOT cached.
func ValidateSourceURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("only https scheme allowed, got %q", u.Scheme)
	}
	host := u.Hostname()
	if net.ParseIP(host) != nil {
		return fmt.Errorf("raw IP literal not allowed: %q", host)
	}
	ips, err := net.LookupHost(host)
	if err != nil {
		return fmt.Errorf("resolve %q: %w", host, err)
	}
	for _, ipStr := range ips {
		parsed := net.ParseIP(ipStr)
		if isReservedIP(parsed) {
			return fmt.Errorf("hostname %q resolves to reserved IP %q", host, ipStr)
		}
	}
	return nil
}

// httpDoer is overridable for tests.
var httpDoer interface {
	Do(*http.Request) (*http.Response, error)
} = &http.Client{Timeout: 30 * time.Second}

// SetHTTPClientForTest overrides the HTTP client in tests.
func SetHTTPClientForTest(c *http.Client) {
	if c == nil {
		httpDoer = &http.Client{Timeout: 30 * time.Second}
	} else {
		httpDoer = c
	}
}

// FetchSource fetches one source URL, writes to cache, and returns the entry.
// If content hash is unchanged from the last cached entry, returns the cached entry
// with Meta.Cached = true.
func FetchSource(ctx context.Context, cacheRoot, provider, sourceID, rawURL string) (*CacheEntry, error) {
	var (
		raw     []byte
		err     error
		lastErr error
	)
	// Exponential backoff: 1s, 2s, 4s, then fail
	delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delays[attempt-1]):
			}
		}
		raw, err = doHTTPFetch(ctx, rawURL)
		if err == nil {
			break
		}
		lastErr = err
	}
	if err != nil {
		return nil, fmt.Errorf("fetch %s after retries: %w", rawURL, lastErr)
	}

	newHash := SHA256Hex(raw)

	// Check if content changed
	if IsCached(cacheRoot, provider, sourceID) {
		existing, readErr := ReadCacheEntry(cacheRoot, provider, sourceID)
		if readErr == nil && existing.Meta.ContentHash == newHash {
			existing.Meta.Cached = true
			return existing, nil
		}
	}

	meta := CacheMeta{
		FetchedAt:   time.Now().UTC(),
		ContentHash: newHash,
		FetchStatus: "ok",
		FetchMethod: "http",
	}
	entry := CacheEntry{
		Provider: provider,
		SourceID: sourceID,
		Raw:      raw,
		Meta:     meta,
	}
	if writeErr := WriteCacheEntry(cacheRoot, entry); writeErr != nil {
		return nil, fmt.Errorf("write cache entry: %w", writeErr)
	}
	return &entry, nil
}

func doHTTPFetch(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "syllago-capmon/1.0")
	resp, err := httpDoer.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func isReservedIP(ip net.IP) bool {
	reserved := []string{
		"127.0.0.0/8",    // loopback
		"169.254.0.0/16", // link-local / AWS IMDS
		"100.64.0.0/10",  // CGNAT / Alibaba IMDS
		"10.0.0.0/8",     // private
		"172.16.0.0/12",  // private
		"192.168.0.0/16", // private
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
	}
	for _, cidr := range reserved {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
