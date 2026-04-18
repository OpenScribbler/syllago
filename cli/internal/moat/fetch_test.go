package moat

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestFetcher_Fetch_Success covers the happy path: 200 response, valid
// manifest body, ETag header preserved, parsed manifest populated.
func TestFetcher_Fetch_Success(t *testing.T) {
	t.Parallel()

	const etag = `"v1"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("User-Agent header missing")
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept header = %q; want application/json", r.Header.Get("Accept"))
		}
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(minimalManifestJSON))
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	res, err := f.Fetch(ctx, srv.URL, "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if res.NotModified {
		t.Error("200 should not set NotModified")
	}
	if res.ETag != etag {
		t.Errorf("ETag = %q; want %q", res.ETag, etag)
	}
	if res.Manifest == nil || res.Manifest.Name != "Example Registry" {
		t.Errorf("Manifest unexpected: %+v", res.Manifest)
	}
	if !bytes.Equal(res.Bytes, []byte(minimalManifestJSON)) {
		t.Error("raw Bytes should match response body verbatim")
	}
	if res.FetchedAt.IsZero() {
		t.Error("FetchedAt should be set")
	}
}

// TestFetcher_Fetch_NotModified verifies If-None-Match is sent and 304
// is surfaced as NotModified=true with nil Manifest/Bytes.
func TestFetcher_Fetch_NotModified(t *testing.T) {
	t.Parallel()

	const etag = `"v1"`
	var gotIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		if gotIfNoneMatch == etag {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(minimalManifestJSON))
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{}
	ctx := context.Background()

	res, err := f.Fetch(ctx, srv.URL, etag)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotIfNoneMatch != etag {
		t.Errorf("server saw If-None-Match = %q; want %q", gotIfNoneMatch, etag)
	}
	if !res.NotModified {
		t.Error("304 should set NotModified = true")
	}
	if res.Manifest != nil || res.Bytes != nil {
		t.Error("NotModified should leave Manifest and Bytes nil")
	}
	if res.ETag != etag {
		t.Errorf("ETag = %q; want %q", res.ETag, etag)
	}
}

// TestFetcher_Fetch_CustomUserAgent verifies the configured UA reaches
// the server unchanged — registry operators use this for analytics.
func TestFetcher_Fetch_CustomUserAgent(t *testing.T) {
	t.Parallel()

	const want = "syllago-test/0.0"
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(minimalManifestJSON))
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{UserAgent: want}
	if _, err := f.Fetch(context.Background(), srv.URL, ""); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotUA != want {
		t.Errorf("User-Agent = %q; want %q", gotUA, want)
	}
}

// TestFetcher_Fetch_DefaultUserAgent verifies the default UA is sent when
// none is configured — registries MAY rely on a syllago- prefix for
// traffic classification.
func TestFetcher_Fetch_DefaultUserAgent(t *testing.T) {
	t.Parallel()

	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(minimalManifestJSON))
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{}
	if _, err := f.Fetch(context.Background(), srv.URL, ""); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.HasPrefix(gotUA, "syllago") {
		t.Errorf("default User-Agent = %q; expected syllago prefix", gotUA)
	}
}

// TestFetcher_Fetch_UnexpectedStatus verifies non-200/304 responses return
// an error rather than silently succeeding.
func TestFetcher_Fetch_UnexpectedStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		code int
	}{
		{"not_found", http.StatusNotFound},
		{"server_error", http.StatusInternalServerError},
		{"forbidden", http.StatusForbidden},
		{"redirect_without_follow", http.StatusPermanentRedirect},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
			}))
			t.Cleanup(srv.Close)

			// Disable automatic redirect following so 308 surfaces as a status error.
			client := &http.Client{
				CheckRedirect: func(*http.Request, []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
			f := &Fetcher{Client: client}
			_, err := f.Fetch(context.Background(), srv.URL, "")
			if err == nil {
				t.Fatalf("expected error for status %d", tt.code)
			}
			if !strings.Contains(err.Error(), http.StatusText(tt.code)) {
				t.Errorf("error = %q; want substring %q", err, http.StatusText(tt.code))
			}
		})
	}
}

// TestFetcher_Fetch_MalformedBody verifies a 200 response with invalid
// manifest bytes returns a ParseManifest error, not a success.
func TestFetcher_Fetch_MalformedBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{}
	_, err := f.Fetch(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected parse error for malformed JSON")
	}
}

// TestFetcher_Fetch_OversizedBody verifies the MaxManifestBytes cap is
// enforced — a registry serving a gigabyte of junk must be rejected
// before parsing.
func TestFetcher_Fetch_OversizedBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write MaxManifestBytes+1 bytes of valid-JSON-ish filler.
		w.Header().Set("Content-Type", "application/json")
		// Prefix with "[" then garbage; size is what matters here.
		filler := bytes.Repeat([]byte("a"), MaxManifestBytes+1)
		_, _ = w.Write(filler)
	}))
	t.Cleanup(srv.Close)

	f := &Fetcher{}
	_, err := f.Fetch(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected oversized body to error")
	}
	if !strings.Contains(err.Error(), "cap") {
		t.Errorf("error = %q; want cap-related message", err)
	}
}

// TestFetcher_Fetch_EmptyURL guards against programmer errors where an
// empty string reaches Fetch (e.g. a config field that was never set).
func TestFetcher_Fetch_EmptyURL(t *testing.T) {
	t.Parallel()

	f := &Fetcher{}
	_, err := f.Fetch(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected empty-URL error")
	}
}

// TestFetcher_Fetch_ContextCancel verifies the context is honored —
// callers MUST be able to cancel an in-flight fetch.
func TestFetcher_Fetch_ContextCancel(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() {
		close(block)
		srv.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	f := &Fetcher{}
	_, err := f.Fetch(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected context-cancel error")
	}
}
