package provmon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckURLs(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
		Slug:            "test",
		ProviderVersion: "v1.0.0",
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases/latest",
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
	if drift.ManifestVersion != "v1.0.0" {
		t.Errorf("ManifestVersion = %q, want %q", drift.ManifestVersion, "v1.0.0")
	}
	if drift.LatestVersion != "v2.0.0" {
		t.Errorf("LatestVersion = %q, want %q", drift.LatestVersion, "v2.0.0")
	}
}

func TestCheckVersion_NoDrift(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.0.0"}`))
	}))
	defer server.Close()

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	m := &Manifest{
		Slug:            "test",
		ProviderVersion: "v1.0.0",
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases/latest",
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

func TestCheckVersion_ContentHash(t *testing.T) {
	t.Parallel()

	// Content-hash method should return nil (not applicable).
	m := &Manifest{
		Slug: "test",
		ChangeDetection: ChangeDetection{
			Method:   "content-hash",
			Endpoint: "https://example.com",
		},
	}

	ctx := context.Background()
	drift, err := CheckVersion(ctx, m)
	if err != nil {
		t.Fatalf("CheckVersion() error: %v", err)
	}
	if drift != nil {
		t.Error("expected nil drift for content-hash method")
	}
}

func TestRunCheck(t *testing.T) {
	t.Parallel()

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
		Slug:            "test",
		DisplayName:     "Test",
		Status:          "active",
		FetchTier:       "gh-api",
		ProviderVersion: "v1.0.0",
		LastVerified:    time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
		ChangeDetection: ChangeDetection{
			Method:   "github-releases",
			Endpoint: server.URL + "/releases",
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
