package provmon

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// newHashedServer returns a test server that serves fixed content for each
// configured path, plus a map of URL → expected sha256 hash. The hash format
// matches capmon.SHA256Hex output ("sha256:<hex>").
func newHashedServer(t *testing.T, paths map[string]string) (*httptest.Server, map[string]string) {
	t.Helper()
	mux := http.NewServeMux()
	for path, body := range paths {
		bodyCopy := body
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/markdown")
			_, _ = w.Write([]byte(bodyCopy))
		})
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	hashes := make(map[string]string, len(paths))
	for path, body := range paths {
		hashes[server.URL+path] = capmon.SHA256Hex([]byte(body))
	}
	return server, hashes
}

type fixtureSource struct {
	URI         string
	FetchMethod string
	ContentHash string
}

// writeFormatDocFixture writes a minimal FormatDoc YAML (capmon's
// provider-formats shape) that provmon will read to find baselines.
func writeFormatDocFixture(t *testing.T, dir, slug string, sources []fixtureSource) {
	t.Helper()
	path := filepath.Join(dir, slug+".yaml")
	content := fmt.Sprintf("provider: %s\ndocs_url: https://example.invalid\ncategory: ide-extension\ncontent_types:\n  rules:\n    status: supported\n    sources:\n", slug)
	for _, s := range sources {
		content += fmt.Sprintf("      - uri: %q\n        type: docs\n        fetch_method: %q\n        content_hash: %q\n        fetched_at: \"2026-04-21T00:00:00Z\"\n",
			s.URI, s.FetchMethod, s.ContentHash)
	}
	content += "    canonical_mappings: {}\n    provider_extensions: []\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckVersion_SourceHash_Stable(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	paths := map[string]string{
		"/rules.md": "# rules body",
		"/hooks.md": "# hooks body",
	}
	server, hashes := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: hashes[server.URL+"/rules.md"]},
		{URI: server.URL + "/hooks.md", FetchMethod: "http", ContentHash: hashes[server.URL+"/hooks.md"]},
	})

	m := &Manifest{
		Slug: "test-provider",
		ChangeDetection: ChangeDetection{
			Method:   "source-hash",
			Endpoint: server.URL,
		},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
			Hooks: ContentType{Sources: []SourceEntry{{URL: server.URL + "/hooks.md", Type: "docs", Format: "markdown"}}},
		},
	}

	ctx := context.Background()
	drift, err := CheckVersionWithFormats(ctx, m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if drift == nil {
		t.Fatal("drift is nil, want VersionDrift with Method=source-hash")
	}
	if drift.Method != "source-hash" {
		t.Errorf("Method = %q, want source-hash", drift.Method)
	}
	if drift.Drifted {
		t.Error("expected no drift")
	}
	if len(drift.Sources) != 2 {
		t.Fatalf("Sources len = %d, want 2", len(drift.Sources))
	}
	for _, s := range drift.Sources {
		if s.Status != StatusStable {
			t.Errorf("source %s: status = %q, want stable (error: %q)", s.URI, s.Status, s.ErrorMessage)
		}
	}
}

func TestCheckVersion_SourceHash_Drifted(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server returns "new content"; FormatDoc records baseline for "old content".
	paths := map[string]string{"/rules.md": "# new content"}
	server, _ := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	oldHash := capmon.SHA256Hex([]byte("# old content"))

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: oldHash},
	})

	m := &Manifest{
		Slug:            "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if drift == nil {
		t.Fatal("drift is nil, want VersionDrift with drift detected")
	}
	if !drift.Drifted {
		t.Error("expected Drifted=true")
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	got := drift.Sources[0]
	if got.Status != StatusDrifted {
		t.Errorf("status = %q, want drifted (error: %q)", got.Status, got.ErrorMessage)
	}
	if got.Baseline != oldHash {
		t.Errorf("Baseline mismatch: got %q, want %q", got.Baseline, oldHash)
	}
	if got.CurrentHash == "" || got.CurrentHash == oldHash {
		t.Errorf("CurrentHash unset or equals baseline: %q", got.CurrentHash)
	}
}

func TestCheckVersion_SourceHash_Skipped(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	paths := map[string]string{"/rules.md": "# content"}
	server, hashes := newHashedServer(t, paths)

	orig := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = orig })

	cases := []struct {
		name         string
		setupFormats func(t *testing.T, dir string)
		wantReason   string
	}{
		{
			name: "formatdoc_missing",
			setupFormats: func(t *testing.T, dir string) {
				// Write nothing — FormatDoc absent.
			},
			wantReason: "FormatDoc missing",
		},
		{
			name: "source_missing",
			setupFormats: func(t *testing.T, dir string) {
				// FormatDoc exists but doesn't list this URL.
				writeFormatDocFixture(t, dir, "test-provider", []fixtureSource{
					{URI: server.URL + "/other.md", FetchMethod: "http", ContentHash: "sha256:deadbeef"},
				})
			},
			wantReason: "missing from FormatDoc",
		},
		{
			name: "baseline_empty",
			setupFormats: func(t *testing.T, dir string) {
				writeFormatDocFixture(t, dir, "test-provider", []fixtureSource{
					{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: ""},
				})
			},
			wantReason: "baseline empty",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			formatsDir := t.TempDir()
			tc.setupFormats(t, formatsDir)

			m := &Manifest{
				Slug:            "test-provider",
				ChangeDetection: ChangeDetection{Method: "source-hash"},
				ContentTypes: ContentTypes{
					Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
				},
			}

			drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
			if err != nil {
				t.Fatalf("CheckVersionWithFormats: %v", err)
			}
			if drift == nil {
				t.Fatal("drift is nil")
			}
			if len(drift.Sources) != 1 {
				t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
			}
			got := drift.Sources[0]
			if got.Status != StatusSkipped {
				t.Errorf("status = %q, want skipped (error: %q)", got.Status, got.ErrorMessage)
			}
			if got.ErrorMessage == "" {
				t.Error("expected non-empty ErrorMessage explaining why skipped")
			}
			if !strings.Contains(got.ErrorMessage, tc.wantReason) {
				t.Errorf("ErrorMessage %q should contain %q", got.ErrorMessage, tc.wantReason)
			}
		})
	}
	_ = hashes // unused across skipped cases
}

func TestCheckVersion_SourceHash_FetchFailed(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server always returns 500.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	t.Cleanup(server.Close)

	orig := httpClient
	httpClient = server.Client()
	capmon.SetHTTPClientForTest(server.Client())
	t.Cleanup(func() {
		httpClient = orig
		capmon.SetHTTPClientForTest(nil)
	})

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: "sha256:baseline"},
	})

	m := &Manifest{
		Slug:            "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	// Short timeout so capmon.FetchSource's 1s+2s+4s retry loop aborts quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	drift, err := CheckVersionWithFormats(ctx, m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if drift == nil {
		t.Fatal("drift is nil")
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	got := drift.Sources[0]
	if got.Status != StatusFetchFailed {
		t.Errorf("status = %q, want fetch_failed (error: %q)", got.Status, got.ErrorMessage)
	}
	if got.ErrorMessage == "" {
		t.Error("expected non-empty fetch_failed ErrorMessage")
	}
	if drift.Drifted {
		t.Error("fetch_failed must not count as drift")
	}
}

func TestCheckVersion_SourceHash_ContentInvalid(t *testing.T) {
	// not t.Parallel() — mutates global httpClient

	// Server returns a 200 login wall — ValidateContentResponse must reject.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><form id="login">Sign in to continue</form></body></html>`))
	}))
	t.Cleanup(server.Close)

	orig := httpClient
	httpClient = server.Client()
	capmon.SetHTTPClientForTest(server.Client())
	t.Cleanup(func() {
		httpClient = orig
		capmon.SetHTTPClientForTest(nil)
	})

	formatsDir := t.TempDir()
	writeFormatDocFixture(t, formatsDir, "test-provider", []fixtureSource{
		{URI: server.URL + "/rules.md", FetchMethod: "http", ContentHash: "sha256:baseline"},
	})

	m := &Manifest{
		Slug:            "test-provider",
		ChangeDetection: ChangeDetection{Method: "source-hash"},
		ContentTypes: ContentTypes{
			Rules: ContentType{Sources: []SourceEntry{{URL: server.URL + "/rules.md", Type: "docs", Format: "markdown"}}},
		},
	}

	drift, err := CheckVersionWithFormats(context.Background(), m, formatsDir)
	if err != nil {
		t.Fatalf("CheckVersionWithFormats: %v", err)
	}
	if drift == nil {
		t.Fatal("drift is nil")
	}
	if len(drift.Sources) != 1 {
		t.Fatalf("Sources len = %d, want 1", len(drift.Sources))
	}
	got := drift.Sources[0]
	if got.Status != StatusContentInvalid {
		t.Errorf("status = %q, want content_invalid (error: %q)", got.Status, got.ErrorMessage)
	}
}
