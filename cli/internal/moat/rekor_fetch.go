package moat

// FetchRekorEntry — per-item Rekor bundle fetcher (bead syllago-ndj5v).
//
// SyncResult carries manifest-level signing metadata but no per-item Rekor
// bundles, so the install path has to fetch them itself before calling
// VerifyAttestationItem. This file is the network-side companion: given a
// logIndex, return the raw Rekor entry JSON exactly as the server emitted
// it.
//
// Bytes are returned verbatim because VerifyAttestationItem hashes and
// re-parses them — any re-marshaling (whitespace trim, field reorder) would
// invalidate the SET signature and fail verification. The caller is the
// only consumer that needs structured access; this layer stays as a byte
// pipe.

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// rekorBaseURL is the production Rekor lookup endpoint. Package-level so
// tests can point at a httptest.NewServer for offline coverage. Restore via
// the withRekorBase test helper.
var rekorBaseURL = "https://rekor.sigstore.dev/api/v1/log/entries"

// rekorFetchClient owns the HTTP timeout for the per-item fetch path. 30s
// is generous for a single small JSON GET — Rekor responses are typically
// a few KB. Keep the timeout below moatFetchClient's 60s tarball budget so
// a slow Rekor doesn't dominate install latency.
var rekorFetchClient = &http.Client{Timeout: 30 * time.Second}

// maxRekorBytes caps the response size to defend against a malicious or
// misbehaving server feeding us an unbounded body. Real entries are
// kilobytes; 10 MiB is far above the legitimate ceiling and well below
// memory-pressure territory.
const maxRekorBytes = 10 << 20

// RekorBaseURLForTest returns the current rekorBaseURL value. Test-only
// surface — production code never reads this. Callers in other packages
// use it to round-trip a save/restore around a temporary override.
func RekorBaseURLForTest() string { return rekorBaseURL }

// SetRekorBaseURLForTest swaps rekorBaseURL for a httptest.NewServer URL.
// Test-only — production callers MUST NOT touch this. Pair every set with
// a t.Cleanup that restores the original via RekorBaseURLForTest.
func SetRekorBaseURLForTest(u string) { rekorBaseURL = u }

// FetchRekorEntry GETs the Rekor entry at the given logIndex and returns
// the response body verbatim. Negative logIndex is rejected before any
// network IO so a malformed manifest entry doesn't generate a noisy 4xx
// upstream. Non-2xx responses, oversize bodies, and context cancellation
// surface as errors.
func FetchRekorEntry(ctx context.Context, logIndex int64) ([]byte, error) {
	if logIndex < 0 {
		return nil, fmt.Errorf("rekor: logIndex must be non-negative, got %d", logIndex)
	}

	url := fmt.Sprintf("%s?logIndex=%d", rekorBaseURL, logIndex)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("rekor: build request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := rekorFetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rekor: fetch logIndex %d: %w", logIndex, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rekor: unexpected status %d for logIndex %d", resp.StatusCode, logIndex)
	}

	// Read one byte past the cap so we can detect overflow without
	// silently truncating the response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRekorBytes+1))
	if err != nil {
		return nil, fmt.Errorf("rekor: read body: %w", err)
	}
	if len(body) > maxRekorBytes {
		return nil, fmt.Errorf("rekor: response exceeds %d byte cap", maxRekorBytes)
	}
	return body, nil
}
