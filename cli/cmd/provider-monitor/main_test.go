package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/provmon"
)

// captureStdout swaps os.Stdout for an os.Pipe, runs fn, then returns the captured output.
// Used to verify printReports formatting without relying on the surrounding test framework.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	w.Close()
	os.Stdout = orig
	return <-done
}

// TestExitCode_DefaultFailOn_DriftedOnly verifies that the default --fail-on=drifted
// policy exits 0 when only fetch_failed / content_invalid / skipped statuses appear,
// and exits non-zero only when drifted appears.
func TestExitCode_DefaultFailOn_DriftedOnly(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		reports  []*provmon.CheckReport
		wantExit int
	}{
		{
			name: "all stable",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusStable},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "fetch_failed but default fail_on",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method: "source-hash",
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusFetchFailed, ErrorMessage: "500"},
					},
				},
			}},
			wantExit: 0,
		},
		{
			name: "drifted triggers exit",
			reports: []*provmon.CheckReport{{
				VersionDrift: &provmon.VersionDrift{
					Method:  "source-hash",
					Drifted: true,
					Sources: []provmon.SourceDrift{
						{Status: provmon.StatusDrifted},
					},
				},
			}},
			wantExit: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := computeExitCode(tc.reports, []string{"drifted"})
			if got != tc.wantExit {
				t.Errorf("computeExitCode = %d, want %d", got, tc.wantExit)
			}
		})
	}
}

// TestExitCode_WideFailOn_FetchFailed verifies that widening --fail-on to
// include fetch_failed flips exit to non-zero when any source fetch-fails,
// while the default --fail-on=drifted policy continues to exit 0.
func TestExitCode_WideFailOn_FetchFailed(t *testing.T) {
	t.Parallel()

	reports := []*provmon.CheckReport{{
		VersionDrift: &provmon.VersionDrift{
			Method: "source-hash",
			Sources: []provmon.SourceDrift{
				{Status: provmon.StatusFetchFailed, ErrorMessage: "500"},
				{Status: provmon.StatusStable},
			},
		},
	}}

	if got := computeExitCode(reports, []string{"drifted"}); got != 0 {
		t.Errorf("default fail_on: got exit %d, want 0", got)
	}

	if got := computeExitCode(reports, []string{"drifted", "fetch_failed"}); got == 0 {
		t.Error("widened fail_on should exit non-zero on fetch_failed")
	}
}

// TestExitCode_FailedURLs verifies that any failed URL forces a non-zero exit
// regardless of --fail-on, since URL health is treated as an unconditional blocker.
func TestExitCode_FailedURLs(t *testing.T) {
	t.Parallel()
	reports := []*provmon.CheckReport{{FailedURLs: 1, TotalURLs: 5}}
	if got := computeExitCode(reports, []string{}); got != 1 {
		t.Errorf("FailedURLs should exit 1 with empty fail_on, got %d", got)
	}
	if got := computeExitCode(reports, []string{"drifted"}); got != 1 {
		t.Errorf("FailedURLs should exit 1 with default fail_on, got %d", got)
	}
}

// TestExitCode_NoVersionDrift confirms reports without VersionDrift skip drift checks.
func TestExitCode_NoVersionDrift(t *testing.T) {
	t.Parallel()
	reports := []*provmon.CheckReport{{Slug: "foo"}, {Slug: "bar"}}
	if got := computeExitCode(reports, []string{"drifted", "fetch_failed"}); got != 0 {
		t.Errorf("nil VersionDrift should exit 0, got %d", got)
	}
}

func TestDaysSinceVerified(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want int
	}{
		{"empty", "", 0},
		{"malformed", "not-a-date", 0},
		{"future date", time.Now().AddDate(0, 0, 5).Format("2006-01-02"), 0},
		{"today", time.Now().Format("2006-01-02"), 0},
		{"ten days ago", time.Now().AddDate(0, 0, -10).Format("2006-01-02"), 10},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := daysSinceVerified(tc.in)
			if tc.name == "future date" {
				if got > 0 {
					t.Errorf("future date should be <= 0, got %d", got)
				}
				return
			}
			// Allow ±1 day slack for clock boundaries.
			if got < tc.want-1 || got > tc.want+1 {
				t.Errorf("daysSinceVerified(%q) = %d, want ~%d", tc.in, got, tc.want)
			}
		})
	}
}

// TestFindManifestDir_Discovers ensures the runtime.Caller path resolves to the
// real docs/provider-sources directory in this repo.
func TestFindManifestDir_Discovers(t *testing.T) {
	t.Parallel()
	dir := findManifestDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("returned dir %q does not exist: %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("returned path %q is not a directory", dir)
	}
	if !strings.HasSuffix(filepath.ToSlash(dir), "docs/provider-sources") {
		t.Errorf("returned dir should end in docs/provider-sources, got %q", dir)
	}
}

func TestPrintReports_StableHasNoNoise(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	reports := []*provmon.CheckReport{{
		Slug:      "foo",
		FetchTier: "stable",
		VersionDrift: &provmon.VersionDrift{
			Method: "source-hash",
			Sources: []provmon.SourceDrift{
				{Status: provmon.StatusStable, ContentType: "rules"},
			},
		},
	}}
	out := captureStdout(t, func() { printReports(reports) })

	if !strings.Contains(out, "foo") || !strings.Contains(out, "OK") {
		t.Errorf("expected slug + OK in output, got:\n%s", out)
	}
	if strings.Contains(out, "DRIFT") || strings.Contains(out, "BROKEN") {
		t.Errorf("stable report should produce no DRIFT/BROKEN lines:\n%s", out)
	}
	if !strings.Contains(out, "1 providers") {
		t.Errorf("expected summary line, got:\n%s", out)
	}
}

func TestPrintReports_BrokenURLs(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	reports := []*provmon.CheckReport{{
		Slug:       "broken",
		FetchTier:  "stable",
		TotalURLs:  3,
		FailedURLs: 2,
		URLResults: []provmon.URLResult{
			{URL: "https://ok.example/a", StatusCode: 200},
			{URL: "https://gone.example/b", StatusCode: 404},
			{URL: "https://err.example/c", Error: errors.New("connection refused")},
		},
	}}
	out := captureStdout(t, func() { printReports(reports) })

	if !strings.Contains(out, "FAIL (2/3 URLs broken)") {
		t.Errorf("expected FAIL line with counts, got:\n%s", out)
	}
	if !strings.Contains(out, "HTTP 404") || !strings.Contains(out, "gone.example") {
		t.Errorf("expected 404 line, got:\n%s", out)
	}
	if !strings.Contains(out, "BROKEN") || !strings.Contains(out, "connection refused") {
		t.Errorf("expected BROKEN error line, got:\n%s", out)
	}
}

func TestPrintReports_GitHubReleasesDrift(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	reports := []*provmon.CheckReport{{
		Slug:      "gh-prov",
		FetchTier: "stable",
		VersionDrift: &provmon.VersionDrift{
			Method:        "github-releases",
			Drifted:       true,
			Baseline:      "v1.0.0",
			LatestVersion: "v1.2.0",
		},
	}}
	out := captureStdout(t, func() { printReports(reports) })

	if !strings.Contains(out, "DRIFT") || !strings.Contains(out, "v1.0.0") || !strings.Contains(out, "v1.2.0") {
		t.Errorf("expected github-releases drift line with baseline+latest, got:\n%s", out)
	}
}

func TestPrintReports_SourceHashAllStatuses(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	reports := []*provmon.CheckReport{{
		Slug:      "src-hash",
		FetchTier: "stable",
		VersionDrift: &provmon.VersionDrift{
			Method: "source-hash",
			Sources: []provmon.SourceDrift{
				{ContentType: "rules", URI: "u1", Status: provmon.StatusStable},
				{ContentType: "rules", URI: "u2", Status: provmon.StatusDrifted, Baseline: "sha256:aaa", CurrentHash: "sha256:bbb"},
				{ContentType: "hooks", URI: "u3", Status: provmon.StatusFetchFailed, ErrorMessage: "timeout"},
				{ContentType: "mcp", URI: "u4", Status: provmon.StatusContentInvalid, ErrorMessage: "bad json"},
				{ContentType: "skills", URI: "u5", Status: provmon.StatusSkipped, ErrorMessage: "no baseline"},
			},
		},
	}}
	out := captureStdout(t, func() { printReports(reports) })

	if !strings.Contains(out, "DRIFT") || !strings.Contains(out, "sha256:aaa") || !strings.Contains(out, "sha256:bbb") {
		t.Errorf("expected drift block with baseline+current, got:\n%s", out)
	}
	if !strings.Contains(out, "FETCH_FAILED") || !strings.Contains(out, "timeout") {
		t.Errorf("expected FETCH_FAILED line, got:\n%s", out)
	}
	if !strings.Contains(out, "CONTENT_INVALID") || !strings.Contains(out, "bad json") {
		t.Errorf("expected CONTENT_INVALID line, got:\n%s", out)
	}
	if !strings.Contains(out, "SKIPPED") {
		t.Errorf("expected SKIPPED line, got:\n%s", out)
	}
	// Summary should reflect counts.
	if !strings.Contains(out, "1 fetch_failed") || !strings.Contains(out, "1 content_invalid") || !strings.Contains(out, "1 skipped") {
		t.Errorf("expected per-status counts in summary, got:\n%s", out)
	}
}

func TestPrintReports_CheckVersionError(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	reports := []*provmon.CheckReport{{
		Slug:              "rate-limited",
		FetchTier:         "stable",
		CheckVersionError: "GitHub API rate limit exceeded",
	}}
	out := captureStdout(t, func() { printReports(reports) })
	if !strings.Contains(out, "CHECK") || !strings.Contains(out, "rate limit") {
		t.Errorf("expected CHECK error line, got:\n%s", out)
	}
}

func TestPrintReports_StaleVerification(t *testing.T) {
	// Cannot run parallel: mutates os.Stdout.
	stale := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	reports := []*provmon.CheckReport{{
		Slug:         "old",
		LastVerified: stale,
	}}
	out := captureStdout(t, func() { printReports(reports) })
	if !strings.Contains(out, "STALE") || !strings.Contains(out, stale) {
		t.Errorf("expected STALE line with date %s, got:\n%s", stale, out)
	}

	fresh := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	freshReports := []*provmon.CheckReport{{Slug: "fresh", LastVerified: fresh}}
	out = captureStdout(t, func() { printReports(freshReports) })
	if strings.Contains(out, "STALE") {
		t.Errorf("fresh date should not produce STALE line, got:\n%s", out)
	}
}

// writeManifest creates a minimal valid manifest YAML at dir/<slug>.yaml.
// Both change_detection.endpoint and the changelog URL feed into AllURLs(), so
// pass the same URL for both unless you want to test a mixed pass/fail report.
func writeManifest(t *testing.T, dir, slug, url string) {
	t.Helper()
	body := "schema_version: \"1\"\n" +
		"slug: " + slug + "\n" +
		"display_name: " + slug + "\n" +
		"vendor: TestCo\n" +
		"status: active\n" +
		"fetch_tier: gh-api\n" +
		"change_detection:\n" +
		"  method: noop\n" +
		"  endpoint: " + url + "\n" +
		"changelog:\n" +
		"  - url: " + url + "\n" +
		"    label: changelog\n" +
		"content_types: {}\n"
	path := filepath.Join(dir, slug+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

// stubServer returns a test server where /ok returns 200 and /broken returns 404.
// Sufficient for driving URL-health paths without hitting the real network.
func stubServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/broken", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRun_HappyPath(t *testing.T) {
	t.Parallel()
	srv := stubServer(t)
	dir := t.TempDir()
	writeManifest(t, dir, "alpha", srv.URL+"/ok")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0\nstderr: %s\nstdout: %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "alpha") {
		t.Errorf("expected slug in stdout, got:\n%s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "1 providers") {
		t.Errorf("expected summary line, got:\n%s", stdout.String())
	}
}

func TestRun_FailedURLForcesNonZero(t *testing.T) {
	t.Parallel()
	srv := stubServer(t)
	dir := t.TempDir()
	writeManifest(t, dir, "broke", srv.URL+"/broken")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1 (404 should force exit 1)\nstdout: %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL") || !strings.Contains(stdout.String(), "HTTP 404") {
		t.Errorf("expected FAIL+HTTP 404 in output, got:\n%s", stdout.String())
	}
}

func TestRun_JSONOutput(t *testing.T) {
	t.Parallel()
	srv := stubServer(t)
	dir := t.TempDir()
	writeManifest(t, dir, "jsontest", srv.URL+"/ok")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-json"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}

	var reports []*provmon.CheckReport
	if err := json.Unmarshal(stdout.Bytes(), &reports); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(reports) != 1 || reports[0].Slug != "jsontest" {
		t.Errorf("expected one report for jsontest, got %+v", reports)
	}
}

func TestRun_ProviderFilter(t *testing.T) {
	t.Parallel()
	srv := stubServer(t)
	dir := t.TempDir()
	writeManifest(t, dir, "alpha", srv.URL+"/ok")
	writeManifest(t, dir, "beta", srv.URL+"/ok")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-provider", "alpha"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "alpha") {
		t.Errorf("expected alpha in output, got:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), "beta ") {
		t.Errorf("filter should have excluded beta, got:\n%s", stdout.String())
	}
}

func TestRun_ProviderFilterNoMatch(t *testing.T) {
	t.Parallel()
	srv := stubServer(t)
	dir := t.TempDir()
	writeManifest(t, dir, "alpha", srv.URL+"/ok")

	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", dir, "-provider", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1 for unknown provider", code)
	}
	if !strings.Contains(stderr.String(), "no manifest found") {
		t.Errorf("expected error in stderr, got:\n%s", stderr.String())
	}
}

func TestRun_ManifestDirMissing(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-dir", "/nonexistent/path/never/exists"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("exit = %d, want 1 for missing dir", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("expected error in stderr, got:\n%s", stderr.String())
	}
}

func TestRun_BadFlag(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	code := run([]string{"-not-a-real-flag"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("exit = %d, want 2 for parse error", code)
	}
}
