package provmon

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckURLs(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Set up test server with some endpoints.
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusPermanentRedirect)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	// Override httpClient to use test server's client.
	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	m := &Manifest{
		ChangeDetection: ChangeDetection{Endpoint: server.URL + "/ok"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{
				{URL: server.URL + "/ok"},
				{URL: server.URL + "/not-found"},
			}},
			Hooks: ContentType{Sources: []SourceEntry{
				{URL: server.URL + "/redirect"},
			}},
		},
	}

	ctx := context.Background()
	results := CheckURLs(ctx, m, 5)

	// 4 total URLs: change_detection + 2 rules + 1 hooks.
	if len(results) != 4 {
		t.Fatalf("CheckURLs returned %d results, want 4", len(results))
	}

	var okCount, failCount int
	for _, r := range results {
		if r.OK() {
			okCount++
		} else {
			failCount++
		}
	}

	if okCount != 3 {
		t.Errorf("OK count = %d, want 3", okCount)
	}
	if failCount != 1 {
		t.Errorf("fail count = %d, want 1", failCount)
	}
}

func TestCheckVersion_GitHubReleases(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Mock GitHub Releases API.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v2.0.0"}`))
	}))
	defer server.Close()

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	m := &Manifest{
		Slug: "test",
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases/latest",
			Baseline: "v1.0.0",
		},
	}

	ctx := context.Background()
	drift, err := CheckVersion(ctx, m)
	if err != nil {
		t.Fatalf("CheckVersion() error: %v", err)
	}

	if !drift.Drifted {
		t.Error("expected drift, got none")
	}
	if drift.Baseline != "v1.0.0" {
		t.Errorf("Baseline = %q, want %q", drift.Baseline, "v1.0.0")
	}
	if drift.LatestVersion != "v2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", drift.LatestVersion, "v2.0.0")
	}
}

func TestCheckVersion_NoDrift(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer server.Close()

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	m := &Manifest{
		Slug: "test",
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases/latest",
			Baseline: "v1.0.0",
		},
	}

	ctx := context.Background()
	drift, err := CheckVersion(ctx, m)
	if err != nil {
		t.Fatalf("CheckVersion() error: %v", err)
	}

	if drift.Drifted {
		t.Error("expected no drift")
	}
}

// TestCheckVersion_UnimplementedMethods ensures that declared-but-unimplemented
// detection methods surface a sentinel error rather than silently returning
// (nil, nil). The previous version of this test pinned the silent no-op and
// would have passed even if detection were removed entirely — see
// syllago-p0phh. A real implementation of either method lives in follow-up
// bead syllago-5gthn; when that lands, update this test to exercise the
// new happy path rather than assert the sentinel.
func TestCheckVersion_UnimplementedMethods(t *testing.T) {
	t.Parallel()

	cases := []struct{ name, method string }{
		{"source-hash stub until Task 19 lands", "source-hash"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			m := &Manifest{
				Slug: "test",
				ChangeDetection: ChangeDetection{
					Method:   tc.method,
					Endpoint: "https://example.com",
				},
			}

			drift, err := CheckVersion(context.Background(), m)
			if drift != nil {
				t.Errorf("regression: CheckVersion returned non-nil drift for unimplemented method %q — that would mean we are silently fabricating drift data; got %+v", tc.method, drift)
			}
			if !errors.Is(err, ErrUnimplementedDetectionMethod) {
				t.Errorf("regression: CheckVersion must return ErrUnimplementedDetectionMethod for %q so callers can distinguish 'no drift' from 'we never tried' — got %v", tc.method, err)
			}
		})
	}
}

// TestCheckVersion_UnknownMethod verifies that a method string we have never
// heard of still returns (nil, nil). That matches the legacy behavior for
// completely unrecognized values — only the two documented-but-unimplemented
// methods get the sentinel. Without this test, we could regress by treating
// "anything not github-releases" as unimplemented and breaking forward
// compatibility with future manifest schema values.
func TestCheckVersion_UnknownMethod(t *testing.T) {
	t.Parallel()

	m := &Manifest{
		Slug: "test",
		ChangeDetection: ChangeDetection{
			Method:   "some-future-method",
			Endpoint: "https://example.com",
		},
	}

	drift, err := CheckVersion(context.Background(), m)
	if err != nil {
		t.Errorf("unknown methods must return (nil, nil) for forward compat, not an error; got %v", err)
	}
	if drift != nil {
		t.Errorf("unknown methods must return nil drift, got %+v", drift)
	}
}

func TestRunCheck(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tag_name": "v2.0.0"}`))
			return
		}
		w.WriteHeader(200)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	m := &Manifest{
		Slug:         "test",
		DisplayName:  "Test",
		Status:       "active",
		FetchTier:    "gh-api",
		LastVerified: time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases",
			Baseline: "v1.0.0",
		},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{
				{URL: server.URL + "/rules"},
			}},
		},
	}

	ctx := context.Background()
	report := RunCheck(ctx, m, 5)

	if report.Slug != "test" {
		t.Errorf("Slug = %q, want %q", report.Slug, "test")
	}
	if report.TotalURLs != 2 { // change_detection + 1 source
		t.Errorf("TotalURLs = %d, want 2", report.TotalURLs)
	}
	if report.FailedURLs != 0 {
		t.Errorf("FailedURLs = %d, want 0", report.FailedURLs)
	}
	if report.VersionDrift == nil {
		t.Fatal("expected version drift")
	}
	if !report.VersionDrift.Drifted {
		t.Error("expected drift")
	}
}
