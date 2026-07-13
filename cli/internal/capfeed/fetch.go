// Package capfeed consumes the Capability Feed published by the external
// Capmon project (https://openscribbler.github.io/capmon/). It implements
// the Capmon Pull side of the contract: polite conditional-GET polling,
// fail-closed provenance verification, and a verbatim mirror of Capability
// Documents into docs/provider-capabilities/.
//
// The fetch layer below is the bytes-typed sibling of moat.Fetcher
// (cli/internal/moat/fetch.go). It is deliberately new code following that
// pattern rather than a generalization of it — see ADR 0016: moat.Fetcher's
// parse step is registry-manifest-specific and MOAT is pinned to "no
// behavior change".
package capfeed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MaxFeedBytes is the hard upper bound on any single feed response. Feed
// files are tens of kilobytes; anything over this limit is malformed or
// malicious and is rejected before full download.
const MaxFeedBytes = 10 << 20 // 10 MiB

// DefaultFetchTimeout is the per-request timeout for feed fetches.
const DefaultFetchTimeout = 30 * time.Second

// DefaultUserAgent is sent when Fetcher.UserAgent is empty. GitHub Pages and
// the GitHub API both require a User-Agent header.
const DefaultUserAgent = "syllago-capmon-pull/1"

// FetchResult carries the outcome of a single Fetch call.
type FetchResult struct {
	// Body is the verbatim response body — the input to hash and signature
	// verification, never re-serialized. nil when NotModified is true.
	Body []byte
	// ETag is the server's ETag header value, if any. Callers persist this
	// value and pass it as prevETag on the next fetch.
	ETag string
	// NotModified is true iff the server returned 304 Not Modified. Body is
	// nil in that case; the caller keeps using its last-known-good copy.
	NotModified bool
	// FetchedAt is the client-clock timestamp of the successful response.
	FetchedAt time.Time
}

// Fetcher performs conditional HTTP GETs against the Capability Feed. The
// zero value is usable; callers may override Client (test substitution) and
// UserAgent.
type Fetcher struct {
	// Client is the HTTP client. nil → a default with DefaultFetchTimeout.
	Client *http.Client
	// UserAgent is sent on every request. Empty → DefaultUserAgent.
	UserAgent string
}

// Fetch performs a conditional GET of url.
//
//   - If prevETag is non-empty, the request includes If-None-Match. A 304
//     response sets result.NotModified = true; Body is nil.
//   - A 200 response returns the raw bytes; no parsing happens here.
//   - Any other status code returns an error.
//   - Responses larger than MaxFeedBytes are rejected.
func (f *Fetcher) Fetch(ctx context.Context, url, prevETag string) (*FetchResult, error) {
	if url == "" {
		return nil, errors.New("capfeed fetch: url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("capfeed fetch %s: build request: %w", url, err)
	}
	req.Header.Set("User-Agent", f.userAgent())
	req.Header.Set("Accept", "application/json")
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("capfeed fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	fetchedAt := time.Now().UTC()

	switch resp.StatusCode {
	case http.StatusNotModified:
		// Drain any body on 304 so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return &FetchResult{
			ETag:        resp.Header.Get("ETag"),
			NotModified: true,
			FetchedAt:   fetchedAt,
		}, nil

	case http.StatusOK:
		// Read at most MaxFeedBytes+1 so an exactly-at-limit payload is
		// still accepted.
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxFeedBytes+1))
		if err != nil {
			return nil, fmt.Errorf("capfeed fetch %s: reading body: %w", url, err)
		}
		if len(body) > MaxFeedBytes {
			return nil, fmt.Errorf("capfeed fetch %s: response exceeds %d-byte cap", url, MaxFeedBytes)
		}
		return &FetchResult{
			Body:        body,
			ETag:        resp.Header.Get("ETag"),
			NotModified: false,
			FetchedAt:   fetchedAt,
		}, nil

	default:
		// Drain a bounded prefix for connection-reuse hygiene.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("capfeed fetch %s: unexpected status %d %s",
			url, resp.StatusCode, http.StatusText(resp.StatusCode))
	}
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return &http.Client{Timeout: DefaultFetchTimeout}
}

func (f *Fetcher) userAgent() string {
	if f.UserAgent != "" {
		return f.UserAgent
	}
	return DefaultUserAgent
}
