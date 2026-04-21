package provmon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrUnimplementedDetectionMethod is returned by CheckVersion when a manifest
// declares a change-detection method that has no implementation in this
// package. Today that covers "content-hash" (used by windsurf, kiro, cursor)
// and "github-commits" (used by amp, copilot-cli). Returning an explicit
// sentinel rather than (nil, nil) is what lets callers and tests tell "no
// drift was detected" apart from "we never even tried to detect drift." See
// syllago-5gthn for the follow-up feature bead that will replace this with
// real detection.
var ErrUnimplementedDetectionMethod = errors.New("change detection method not implemented")

// httpClient is overridable for tests.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// URLResult holds the outcome of checking a single URL.
type URLResult struct {
	URL        string
	StatusCode int
	Error      error
}

// OK returns true if the URL returned a 2xx or 3xx status.
func (r URLResult) OK() bool {
	return r.Error == nil && r.StatusCode >= 200 && r.StatusCode < 400
}

// CheckReport is the full report for one provider manifest.
type CheckReport struct {
	Slug         string
	DisplayName  string
	Status       string // active | archived | beta
	FetchTier    string
	URLResults   []URLResult
	VersionDrift *VersionDrift // nil if change detection not applicable
	TotalURLs    int
	FailedURLs   int
	LastVerified string
	Baseline     string
}

// VersionDrift describes when the provider's latest version differs from what was last verified.
type VersionDrift struct {
	Baseline      string // what the manifest records (version tag, for github-releases)
	LatestVersion string // what the API says
	Drifted       bool
}

// CheckURLs performs concurrent HEAD requests against all URLs in a manifest.
// Concurrency is capped at maxConcurrent.
func CheckURLs(ctx context.Context, m *Manifest, maxConcurrent int) []URLResult {
	urls := m.AllURLs()
	results := make([]URLResult, len(urls))

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup

	for i, u := range urls {
		wg.Add(1)
		go func(idx int, url string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			results[idx] = checkOneURL(ctx, url)
		}(i, u)
	}

	wg.Wait()
	return results
}

func checkOneURL(ctx context.Context, url string) URLResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return URLResult{URL: url, Error: fmt.Errorf("creating request: %w", err)}
	}
	// GitHub API requires a User-Agent header.
	req.Header.Set("User-Agent", "syllago-provider-monitor/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return URLResult{URL: url, Error: err}
	}
	_ = resp.Body.Close()

	// Some servers don't support HEAD well — fall back to GET if we get 405.
	if resp.StatusCode == http.StatusMethodNotAllowed {
		req.Method = http.MethodGet
		resp, err = httpClient.Do(req)
		if err != nil {
			return URLResult{URL: url, Error: err}
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	return URLResult{URL: url, StatusCode: resp.StatusCode}
}

// CheckVersion queries the provider's change-detection endpoint to detect
// drift. Only the "github-releases" method is implemented; the other method
// values documented in manifest.go (content-hash, github-commits) return
// ErrUnimplementedDetectionMethod so callers can distinguish "no drift"
// from "unsupported method."
func CheckVersion(ctx context.Context, m *Manifest) (*VersionDrift, error) {
	switch m.ChangeDetection.Method {
	case "github-releases":
		// fall through to the implementation below
	case "source-hash":
		return nil, fmt.Errorf("%w: %s", ErrUnimplementedDetectionMethod, m.ChangeDetection.Method)
	default:
		return nil, nil
	}

	endpoint := m.ChangeDetection.Endpoint
	if endpoint == "" {
		return nil, fmt.Errorf("no change detection endpoint for %s", m.Slug)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "syllago-provider-monitor/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases for %s: %w", m.Slug, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("releases API for %s returned %d", m.Slug, resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding releases for %s: %w", m.Slug, err)
	}

	return &VersionDrift{
		Baseline:      m.ChangeDetection.Baseline,
		LatestVersion: release.TagName,
		Drifted:       m.ChangeDetection.Baseline != "" && release.TagName != m.ChangeDetection.Baseline,
	}, nil
}

// RunCheck performs a full check on a single manifest: URL health + version drift.
func RunCheck(ctx context.Context, m *Manifest, maxConcurrent int) *CheckReport {
	urlResults := CheckURLs(ctx, m, maxConcurrent)

	var failed int
	for _, r := range urlResults {
		if !r.OK() {
			failed++
		}
	}

	report := &CheckReport{
		Slug:         m.Slug,
		DisplayName:  m.DisplayName,
		Status:       m.Status,
		FetchTier:    m.FetchTier,
		URLResults:   urlResults,
		TotalURLs:    len(urlResults),
		FailedURLs:   failed,
		LastVerified: m.LastVerified,
		Baseline:     m.ChangeDetection.Baseline,
	}

	drift, err := CheckVersion(ctx, m)
	if err == nil {
		report.VersionDrift = drift
	}
	// Version check failures are non-fatal — URL results are still useful.

	return report
}
