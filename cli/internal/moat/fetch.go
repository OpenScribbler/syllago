package moat

// MOAT registry manifest HTTP fetch (spec §Registry Manifest). This layer
// only knows how to GET a manifest URL and obey HTTP caching semantics —
// it does NOT verify signatures. Signature verification runs on the raw
// bytes returned here; see sigstore_verify.go.
//
// Caching rationale (Panel C7): manifests can run 3-6 MB. Most fetches are
// polls that want to know "did anything change?". ETag + If-None-Match lets
// the server answer 304 without transferring the body, reducing bandwidth
// by ~99% when the manifest hasn't changed. Clients persist the ETag per
// registry URL; the value is opaque.
//
// Size cap (MaxManifestBytes): a 50 MiB ceiling guards against malicious
// or runaway registries. Real manifests are orders of magnitude smaller;
// anything over this limit is rejected before full download.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MaxManifestBytes is the hard upper bound on manifest size. A registry
// larger than this is malformed or malicious — reject without parsing.
const MaxManifestBytes = 50 << 20 // 50 MiB

// DefaultFetchTimeout is the per-request timeout for manifest fetches.
const DefaultFetchTimeout = 30 * time.Second

// DefaultUserAgent is sent when Fetcher.UserAgent is empty.
const DefaultUserAgent = "syllago-moat/1"

// FetchResult carries the outcome of a single Fetch call.
type FetchResult struct {
	// Manifest is the parsed, validated manifest. nil when NotModified is true.
	Manifest *Manifest
	// Bytes is the verbatim response body — the input to signature
	// verification. Storing the raw bytes (not re-marshaled JSON) is
	// mandatory because any re-serialization would invalidate the signature.
	// nil when NotModified is true.
	Bytes []byte
	// ETag is the server's ETag header value, if any. Callers persist this
	// value per-registry and pass it as prevETag on the next fetch.
	ETag string
	// NotModified is true iff the server returned 304 Not Modified.
	// Manifest and Bytes are nil in that case; the caller should continue
	// using its previously cached copy.
	NotModified bool
	// FetchedAt is the client-clock timestamp of the successful response.
	// Written into the lockfile's registries[url].fetched_at field for
	// staleness enforcement (see G-9).
	FetchedAt time.Time
}

// Fetcher performs conditional HTTP GETs for MOAT registry manifests.
// The zero value is usable; callers may override Client (for test
// substitution) and UserAgent.
type Fetcher struct {
	// Client is the HTTP client. nil → a default with DefaultFetchTimeout.
	Client *http.Client
	// UserAgent is sent on every request. Empty → DefaultUserAgent.
	UserAgent string
}

// Fetch performs a conditional GET of the manifest at url.
//
//   - If prevETag is non-empty, the request includes If-None-Match. A 304
//     response sets result.NotModified = true; Manifest and Bytes are nil.
//   - A 200 response is parsed and validated; malformed manifests return an
//     error rather than a populated result.
//   - Any other status code returns an error.
//   - Responses larger than MaxManifestBytes are rejected.
//
// The body is always fully drained (or explicitly limited) so the
// underlying connection can be reused.
func (f *Fetcher) Fetch(ctx context.Context, url, prevETag string) (*FetchResult, error) {
	if url == "" {
		return nil, errors.New("fetch: url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: build request: %w", url, err)
	}
	req.Header.Set("User-Agent", f.userAgent())
	req.Header.Set("Accept", "application/json")
	if prevETag != "" {
		req.Header.Set("If-None-Match", prevETag)
	}

	resp, err := f.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
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
		// Guard against oversized bodies — read at most MaxManifestBytes+1
		// so an exactly-at-limit payload is still accepted.
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxManifestBytes+1))
		if err != nil {
			return nil, fmt.Errorf("fetch %s: reading body: %w", url, err)
		}
		if len(body) > MaxManifestBytes {
			return nil, fmt.Errorf("fetch %s: manifest exceeds %d-byte cap", url, MaxManifestBytes)
		}
		mf, err := ParseManifest(body)
		if err != nil {
			return nil, fmt.Errorf("fetch %s: %w", url, err)
		}
		return &FetchResult{
			Manifest:    mf,
			Bytes:       body,
			ETag:        resp.Header.Get("ETag"),
			NotModified: false,
			FetchedAt:   fetchedAt,
		}, nil

	default:
		// Drain a bounded prefix for error-reporting hygiene.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil, fmt.Errorf("fetch %s: unexpected status %d %s",
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
