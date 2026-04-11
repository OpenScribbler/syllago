package capmon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// CapmonCheckOptions configures a RunCapmonCheck run.
type CapmonCheckOptions struct {
	// ProvidersJSON is the path to providers.json (default: "providers.json").
	ProvidersJSON string
	// FormatsDir is the directory containing provider format doc YAML files
	// (default: "docs/provider-formats").
	FormatsDir string
	// SourcesDir is the directory containing provider source manifests
	// (default: "docs/provider-sources").
	SourcesDir string
	// CacheRoot is the root of the capmon cache (default: ".capmon-cache").
	CacheRoot string
	// CanonicalKeysPath is the path to canonical-keys.yaml
	// (default: "docs/spec/canonical-keys.yaml").
	CanonicalKeysPath string
	// ProviderFilter limits the run to a single provider slug. Empty means all.
	ProviderFilter string
	// DryRun logs actions but makes no GitHub API calls.
	DryRun bool
}

// providersDoc is the minimal shape of providers.json needed for orphan detection.
type providersDoc struct {
	Providers []struct {
		Slug string `json:"slug"`
	} `json:"providers"`
}

// loadProviderSlugs parses providers.json and returns the set of known slugs.
func loadProviderSlugs(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read providers.json %s: %w", path, err)
	}
	var doc providersDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse providers.json: %w", err)
	}
	slugs := make(map[string]bool, len(doc.Providers))
	for _, p := range doc.Providers {
		slugs[p.Slug] = true
	}
	return slugs, nil
}

// RunCapmonCheck runs the capmon check pipeline over all (or one filtered) provider
// format docs. It validates infrastructure, detects content drift, and creates or
// updates GitHub issues for each changed source.
//
// Pipeline:
//  0. Load providers.json; warn on orphan format docs (non-blocking)
//  1. ValidateSources for each provider (blocking)
//  2. ValidateFormatDoc for each format doc (blocking)
//  3. Fetch each source, compare hash, create/append issue on change
func RunCapmonCheck(ctx context.Context, opts CapmonCheckOptions) error {
	// Apply defaults.
	if opts.ProvidersJSON == "" {
		opts.ProvidersJSON = "providers.json"
	}
	if opts.FormatsDir == "" {
		opts.FormatsDir = "docs/provider-formats"
	}
	if opts.SourcesDir == "" {
		opts.SourcesDir = "docs/provider-sources"
	}
	if opts.CacheRoot == "" {
		opts.CacheRoot = ".capmon-cache"
	}
	if opts.CanonicalKeysPath == "" {
		opts.CanonicalKeysPath = "docs/spec/canonical-keys.yaml"
	}

	// Load known slugs for orphan detection.
	knownSlugs, err := loadProviderSlugs(opts.ProvidersJSON)
	if err != nil {
		return err
	}

	// Enumerate format doc files.
	entries, err := os.ReadDir(opts.FormatsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // empty dir is valid
		}
		return fmt.Errorf("read formats dir: %w", err)
	}

	var providers []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".yaml")
		if opts.ProviderFilter != "" && slug != opts.ProviderFilter {
			continue
		}
		providers = append(providers, slug)
	}

	for _, provider := range providers {
		// Step 0: Orphan detection (non-blocking warning).
		if !knownSlugs[provider] {
			fmt.Fprintf(os.Stderr, "warning: format doc for %q has no entry in providers.json (orphan)\n", provider)
		}

		// Step 1: Validate source manifest (blocking).
		if err := ValidateSources(opts.SourcesDir, provider); err != nil {
			return fmt.Errorf("capmon check: %w", err)
		}

		// Step 2: Validate format doc (blocking).
		if err := ValidateFormatDoc(opts.FormatsDir, opts.CanonicalKeysPath, provider); err != nil {
			return fmt.Errorf("capmon check: validate format doc for %s: %w", provider, err)
		}

		// Step 3: Fetch and compare each source URI.
		doc, err := LoadFormatDoc(FormatDocPath(opts.FormatsDir, provider))
		if err != nil {
			return fmt.Errorf("capmon check: load format doc for %s: %w", provider, err)
		}

		for ct, ctDoc := range doc.ContentTypes {
			for _, src := range ctDoc.Sources {
				if err := runSourceCheck(ctx, opts, provider, ct, src); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// runSourceCheck fetches one source URI, validates the response, compares the hash
// against the stored value in the format doc, and creates or appends a GitHub issue
// if the content has changed.
func runSourceCheck(ctx context.Context, opts CapmonCheckOptions, provider, contentType string, src SourceRef) error {
	// Fetch content.
	body, respContentType, finalURL, fetchErr := fetchForCheck(ctx, src.URI)
	if fetchErr != nil {
		logOrCreateFetchErrorIssue(ctx, opts, provider, contentType, src.URI,
			fmt.Sprintf("fetch error: %v", fetchErr))
		return nil
	}

	// Validate content response.
	if err := ValidateContentResponse(body, respContentType, src.URI, finalURL); err != nil {
		logOrCreateFetchErrorIssue(ctx, opts, provider, contentType, src.URI,
			fmt.Sprintf("content invalid: %v", err))
		return nil
	}

	// Compare hash.
	newHash := SHA256Hex(body)
	if src.ContentHash != "" && src.ContentHash == newHash {
		return nil // no change
	}

	// Content changed (or first fetch — empty hash).
	message := fmt.Sprintf("Content hash changed for %s/%s source %s:\nOld hash: %s\nNew hash: %s",
		provider, contentType, src.URI, src.ContentHash, newHash)

	if opts.DryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would create/append capmon-change issue for %s/%s (source %s)\n",
			provider, contentType, src.URI)
		return nil
	}

	// Find or create issue.
	issueNum, found, err := FindOpenCapmonIssue(provider, contentType)
	if err != nil {
		// Non-blocking: log and continue.
		fmt.Fprintf(os.Stderr, "warning: find issue for %s/%s: %v\n", provider, contentType, err)
		return nil
	}

	title := fmt.Sprintf("capmon: content change detected for %s/%s", provider, contentType)
	if found {
		return AppendCapmonChangeEvent(ctx, issueNum, message)
	}
	_, err = CreateCapmonChangeIssue(ctx, provider, contentType, title, message)
	return err
}

// logOrCreateFetchErrorIssue creates a GitHub issue for a fetch/validity failure,
// or logs to stderr when in dry-run mode.
func logOrCreateFetchErrorIssue(ctx context.Context, opts CapmonCheckOptions, provider, contentType, sourceURI, reason string) {
	if opts.DryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would create capmon-fetch-error issue for %s/%s (%s): %s\n",
			provider, contentType, sourceURI, reason)
		return
	}
	slug, _ := SanitizeSlug(provider)
	_, _ = ghRunner("issue", "create",
		"--title", fmt.Sprintf("capmon: fetch error for %s/%s", slug, contentType),
		"--label", "capmon-fetch-error",
		"--label", "provider:"+slug,
		"--body", fmt.Sprintf("Source URI: %s\nReason: %s", sourceURI, reason),
	)
}

// fetchForCheck makes a direct HTTP GET and returns the body, Content-Type header,
// final URL (after redirects), and any error. Uses the same httpDoer as FetchSource
// so it is overridable in tests via SetHTTPClientForTest.
func fetchForCheck(ctx context.Context, rawURL string) (body []byte, contentType, finalURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "syllago-capmon/1.0")
	resp, err := httpDoer.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", fmt.Errorf("read body: %w", err)
	}
	ct := resp.Header.Get("Content-Type")
	// Final URL: use the request URL from the response (set by http.Client after redirects).
	fu := rawURL
	if resp.Request != nil && resp.Request.URL != nil {
		fu = resp.Request.URL.String()
	}
	return body, ct, fu, nil
}
