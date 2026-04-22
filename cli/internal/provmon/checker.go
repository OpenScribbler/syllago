package provmon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

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
	Slug              string
	DisplayName       string
	Status            string // active | archived | beta
	FetchTier         string
	URLResults        []URLResult
	VersionDrift      *VersionDrift // nil if change detection not applicable
	TotalURLs         int
	FailedURLs        int
	LastVerified      string
	Baseline          string
	CheckVersionError string // empty on success; set when CheckVersion returned a setup-level error
}

// VersionDrift describes when the provider's latest version differs from what was last verified.
type VersionDrift struct {
	Method        string // detection method from the manifest ("github-releases" or "source-hash")
	Baseline      string // what the manifest records (version tag, for github-releases)
	LatestVersion string // what the API says
	Drifted       bool
	Sources       []SourceDrift // per-source results when Method == "source-hash"
}

// SourceDriftStatus classifies the result of checking a single source URL for
// content drift. Exactly one status applies per source per check run.
type SourceDriftStatus string

const (
	// StatusStable: fetched body's sha256 matches the manifest baseline. No drift.
	StatusStable SourceDriftStatus = "stable"
	// StatusDrifted: fetched body's sha256 differs from the manifest baseline. Drift detected.
	StatusDrifted SourceDriftStatus = "drifted"
	// StatusSkipped: baseline is empty in the manifest (capture-on-first-check),
	// so we can't compare yet. The fetched hash is recorded for future runs.
	StatusSkipped SourceDriftStatus = "skipped"
	// StatusFetchFailed: HTTP/transport error prevented computing a hash for this source.
	StatusFetchFailed SourceDriftStatus = "fetch_failed"
	// StatusContentInvalid: body was fetched but couldn't be decoded (e.g., malformed JSON
	// for github-commits endpoints). Reserved for method-specific semantic failures.
	StatusContentInvalid SourceDriftStatus = "content_invalid"
)

// SourceDrift is the result of checking a single source URL for content drift.
type SourceDrift struct {
	ContentType  string // rules | hooks | mcp | skills | agents | commands
	URI          string
	Baseline     string            // sha256:... from the manifest (may be empty)
	CurrentHash  string            // sha256:... computed from the fetched body (empty on fetch/parse failure)
	Status       SourceDriftStatus // exactly one of the constants above
	ErrorMessage string            // populated when Status is fetch_failed, content_invalid, or skipped (explains why)
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

// CheckVersion is the default entry used by RunCheck; resolves formatsDir from
// the current working directory (repo root or cli/ parent).
func CheckVersion(ctx context.Context, m *Manifest) (*VersionDrift, error) {
	return CheckVersionWithFormats(ctx, m, defaultFormatsDir())
}

// CheckVersionWithFormats dispatches drift detection by method. Tests pass an
// explicit formatsDir so they don't depend on repo layout; the CLI uses
// defaultFormatsDir for the real docs/provider-formats/ directory.
func CheckVersionWithFormats(ctx context.Context, m *Manifest, formatsDir string) (*VersionDrift, error) {
	switch m.ChangeDetection.Method {
	case "github-releases":
		return checkGithubReleases(ctx, m)
	case "source-hash":
		return checkSourceHash(ctx, m, formatsDir)
	default:
		return nil, nil
	}
}

// checkGithubReleases queries the github-releases API and compares the latest
// release tag to the manifest baseline.
func checkGithubReleases(ctx context.Context, m *Manifest) (*VersionDrift, error) {
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
		Method:        "github-releases",
		Baseline:      m.ChangeDetection.Baseline,
		LatestVersion: release.TagName,
		Drifted:       m.ChangeDetection.Baseline != "" && release.TagName != m.ChangeDetection.Baseline,
	}, nil
}

// defaultFormatsDir resolves docs/provider-formats relative to the current
// working directory. Tries cwd/, then cwd/../ (handles running from cli/).
func defaultFormatsDir() string {
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "docs", "provider-formats")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	candidate = filepath.Join(cwd, "..", "docs", "provider-formats")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return filepath.Join("docs", "provider-formats")
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
	// Always populate VersionDrift even when err is non-nil — source-hash
	// drift objects hold per-source results that remain useful even if
	// one source transport-failed.
	report.VersionDrift = drift
	if err != nil {
		// Setup-level error (e.g., github-releases API 500, FormatDoc
		// parse error). Surface it as a field on the report; non-fatal
		// to URL health results.
		report.CheckVersionError = err.Error()
	}

	return report
}
