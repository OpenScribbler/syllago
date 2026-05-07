package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"github.com/OpenScribbler/syllago/cli/internal/output"
)

// writeTestSourceManifest creates a minimal provider-sources YAML with nSources entries
// under content_types.rules.sources in dir/<slug>.yaml.
func writeTestSourceManifest(t *testing.T, dir, slug string, nSources int) {
	t.Helper()
	var sb strings.Builder
	fmt.Fprintf(&sb, "schema_version: \"1\"\nslug: %s\ndisplay_name: %s\ncontent_types:\n  rules:\n    sources:\n", slug, slug)
	for i := 0; i < nSources; i++ {
		fmt.Fprintf(&sb, "      - url: https://example.com/%s/doc-%d\n        type: docs\n        format: markdown\n", slug, i)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write test source manifest %s: %v", slug, err)
	}
}

// writeTestSourceManifestWithURLs creates a provider-sources YAML where all sources
// point to the given base URL. Used for live-fetch tests with httptest servers.
func writeTestSourceManifestWithURLs(t *testing.T, dir, slug, baseURL string, nSources int) {
	t.Helper()
	var sb strings.Builder
	fmt.Fprintf(&sb, "schema_version: \"1\"\nslug: %s\ndisplay_name: %s\ncontent_types:\n  rules:\n    sources:\n", slug, slug)
	for i := 0; i < nSources; i++ {
		fmt.Fprintf(&sb, "      - url: %s/doc-%d\n        type: docs\n        format: markdown\n", baseURL, i)
	}
	if err := os.WriteFile(filepath.Join(dir, slug+".yaml"), []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write test source manifest %s: %v", slug, err)
	}
}

// TestCapmonFetchCmd_DryRun_PrintsSourceCounts verifies that --dry-run reports
// the total source count per provider without writing any cache files.
func TestCapmonFetchCmd_DryRun_PrintsSourceCounts(t *testing.T) {
	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 3)

	stdout, _ := output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("output missing provider slug %q; got:\n%s", "alpha-provider", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("output missing source count 3; got:\n%s", out)
	}

	// Dry-run must not write any cache entries.
	entries, _ := os.ReadDir(cacheDir)
	if len(entries) != 0 {
		t.Errorf("dry-run wrote %d cache entries, want 0", len(entries))
	}
}

// TestCapmonFetchCmd_DryRun_ProviderFilter verifies that --provider restricts
// the dry-run report to the matched slug only.
func TestCapmonFetchCmd_DryRun_ProviderFilter(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 2)
	writeTestSourceManifest(t, srcDir, "beta-provider", 5)

	stdout, _ := output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("provider", "alpha-provider")
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("provider", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("output missing alpha-provider; got:\n%s", out)
	}
	if strings.Contains(out, "beta-provider") {
		t.Errorf("filtered output must not mention beta-provider; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_DryRun_UnknownProvider verifies that --provider set to a
// valid-format slug that has no manifest returns an error listing valid slugs.
func TestCapmonFetchCmd_DryRun_UnknownProvider(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 1)

	_, _ = output.SetForTest(t)
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("provider", "unknown-provider")
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("provider", "")
	}()

	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "alpha-provider") {
		t.Errorf("error must list valid provider slugs so the user can self-correct; got: %v", err)
	}
}

// TestCapmonFetchCmd_DryRun_InvalidSlugFormat verifies that a --provider value
// with illegal characters surfaces "invalid --provider" (not a generic "not implemented").
func TestCapmonFetchCmd_DryRun_InvalidSlugFormat(t *testing.T) {
	_, _ = output.SetForTest(t)
	capmonFetchCmd.Flags().Set("provider", "INVALID SLUG")
	defer capmonFetchCmd.Flags().Set("provider", "")

	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Fatal("expected error for invalid slug format, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --provider") {
		t.Errorf("expected error to mention %q; got: %v", "invalid --provider", err)
	}
}

// TestCapmonFetchCmd_DryRun_JSON verifies that --dry-run with output.JSON=true
// emits valid JSON containing the provider slug and correct source count.
func TestCapmonFetchCmd_DryRun_JSON(t *testing.T) {
	srcDir := t.TempDir()
	writeTestSourceManifest(t, srcDir, "alpha-provider", 4)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	capmonFetchCmd.Flags().Set("dry-run", "true")
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	defer func() {
		capmonFetchCmd.Flags().Set("dry-run", "false")
		capmonFetchCmd.Flags().Set("sources-dir", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := strings.TrimSpace(stdout.String())
	var payload interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "alpha-provider") {
		t.Errorf("JSON output missing provider slug; got:\n%s", out)
	}
	if !strings.Contains(out, "4") {
		t.Errorf("JSON output missing source count 4; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_LiveFetch_SummaryOutput verifies that live fetch (no --dry-run)
// prints a per-provider summary line with fetched count and zero errors.
func TestCapmonFetchCmd_LiveFetch_SummaryOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "provider docs content for "+r.URL.Path)
	}))
	defer ts.Close()

	capmon.SetValidateURLForTest(func(string) error { return nil })
	defer capmon.SetValidateURLForTest(nil)

	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifestWithURLs(t, srcDir, "live-provider", ts.URL, 1)

	stdout, _ := output.SetForTest(t)
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "live-provider") {
		t.Errorf("summary missing provider slug; got:\n%s", out)
	}
	// Should report 1 fetched and 0 errors.
	if !strings.Contains(out, "1") {
		t.Errorf("summary missing fetch count 1; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_LiveFetch_VerboseOutput verifies that --verbose adds per-source
// detail lines with a source ID prefix and a [changed] or [cached] indicator.
func TestCapmonFetchCmd_LiveFetch_VerboseOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "live provider docs for "+r.URL.Path)
	}))
	defer ts.Close()

	capmon.SetValidateURLForTest(func(string) error { return nil })
	defer capmon.SetValidateURLForTest(nil)

	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifestWithURLs(t, srcDir, "verbose-provider", ts.URL, 1)

	stdout, _ := output.SetForTest(t)
	output.Verbose = true
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := stdout.String()
	// Verbose mode must include per-source lines: source ID (e.g. rules.0) and status.
	if !strings.Contains(out, "rules.0") {
		t.Errorf("verbose output missing source ID rules.0; got:\n%s", out)
	}
	if !strings.Contains(out, "[changed]") && !strings.Contains(out, "[cached]") {
		t.Errorf("verbose output missing [changed] or [cached] indicator; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_LiveFetch_ExitNonZeroOnError verifies that RunE returns
// non-nil when all sources fail to fetch. Uses a short context timeout to avoid
// the full 1s+2s+4s retry delay while still exercising the error-propagation path.
func TestCapmonFetchCmd_LiveFetch_ExitNonZeroOnError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	capmon.SetValidateURLForTest(func(string) error { return nil })
	defer capmon.SetValidateURLForTest(nil)

	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifestWithURLs(t, srcDir, "err-provider", ts.URL, 1)

	_, _ = output.SetForTest(t)

	// Short timeout to cancel the retry sleep after the first failed attempt.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	capmonFetchCmd.SetContext(ctx)
	defer capmonFetchCmd.SetContext(context.Background())

	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	err := capmonFetchCmd.RunE(capmonFetchCmd, []string{})
	if err == nil {
		t.Fatal("expected non-nil error when all sources fail, got nil")
	}
}

// TestCapmonFetchCmd_LiveFetch_JSONOutput verifies that live fetch with output.JSON=true
// emits valid JSON with a providers map containing fetched/cached/errors keys.
func TestCapmonFetchCmd_LiveFetch_JSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "json test content")
	}))
	defer ts.Close()

	capmon.SetValidateURLForTest(func(string) error { return nil })
	defer capmon.SetValidateURLForTest(nil)

	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifestWithURLs(t, srcDir, "json-provider", ts.URL, 2)

	stdout, _ := output.SetForTest(t)
	output.JSON = true
	capmonFetchCmd.Flags().Set("sources-dir", srcDir)
	capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	defer func() {
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}()

	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	out := strings.TrimSpace(stdout.String())
	var payload interface{}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("JSON output invalid: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "json-provider") {
		t.Errorf("JSON output missing provider slug; got:\n%s", out)
	}
	// Must include per-provider counts (fetched/cached/errors).
	if !strings.Contains(out, "fetched") || !strings.Contains(out, "errors") {
		t.Errorf("JSON output missing fetched/errors keys; got:\n%s", out)
	}
}

// TestCapmonFetchCmd_LiveFetch_AllCached_ExitZero verifies that a second fetch
// of deterministic content returns nil (all sources already cached, no errors).
func TestCapmonFetchCmd_LiveFetch_AllCached_ExitZero(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "deterministic content — same every request")
	}))
	defer ts.Close()

	capmon.SetValidateURLForTest(func(string) error { return nil })
	defer capmon.SetValidateURLForTest(nil)

	srcDir := t.TempDir()
	cacheDir := t.TempDir()
	writeTestSourceManifestWithURLs(t, srcDir, "cached-provider", ts.URL, 1)

	setFlags := func() {
		capmonFetchCmd.Flags().Set("sources-dir", srcDir)
		capmonFetchCmd.Flags().Set("cache-root", cacheDir)
	}
	clearFlags := func() {
		capmonFetchCmd.Flags().Set("sources-dir", "")
		capmonFetchCmd.Flags().Set("cache-root", "")
	}

	// First run: populates cache.
	_, _ = output.SetForTest(t)
	setFlags()
	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		clearFlags()
		t.Fatalf("first RunE: %v", err)
	}
	clearFlags()

	// Second run: all sources hit cache, should still succeed (nil error).
	stdout2, _ := output.SetForTest(t)
	setFlags()
	defer clearFlags()
	if err := capmonFetchCmd.RunE(capmonFetchCmd, []string{}); err != nil {
		t.Fatalf("second RunE (all-cached): %v", err)
	}

	out := stdout2.String()
	if !strings.Contains(out, "cached-provider") {
		t.Errorf("second-run output missing provider slug; got:\n%s", out)
	}
}
