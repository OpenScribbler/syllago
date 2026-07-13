package capfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch_ConditionalGET(t *testing.T) {
	const etag = `"rev-1"`
	body := `{"data_revision":"abc"}`

	var sawIfNoneMatch string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawIfNoneMatch = r.Header.Get("If-None-Match")
		if sawIfNoneMatch == etag {
			// Real servers (incl. GitHub Pages) echo the ETag on 304.
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	f := &Fetcher{}

	// First fetch: no previous ETag, expect a 200 with body + ETag.
	res, err := f.Fetch(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatalf("first Fetch: %v", err)
	}
	if sawIfNoneMatch != "" {
		t.Errorf("first request sent If-None-Match %q; want none", sawIfNoneMatch)
	}
	if res.NotModified {
		t.Error("first fetch reported NotModified; want full response")
	}
	if string(res.Body) != body {
		t.Errorf("Body = %q; want %q", res.Body, body)
	}
	if res.ETag != etag {
		t.Errorf("ETag = %q; want %q", res.ETag, etag)
	}
	if res.FetchedAt.IsZero() {
		t.Error("FetchedAt is zero; want a timestamp")
	}

	// Second fetch: previous ETag supplied, expect 304 → NotModified, no body.
	res, err = f.Fetch(context.Background(), srv.URL, etag)
	if err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if sawIfNoneMatch != etag {
		t.Errorf("second request If-None-Match = %q; want %q", sawIfNoneMatch, etag)
	}
	if !res.NotModified {
		t.Error("second fetch NotModified = false; want true")
	}
	if res.ETag != etag {
		t.Errorf("304 result ETag = %q; want server ETag %q (callers persist this as the next cache key)", res.ETag, etag)
	}
	if res.Body != nil {
		t.Errorf("304 result carried a body of %d bytes; want nil", len(res.Body))
	}
}

func TestFetch_SetsUserAgentAndAccept(t *testing.T) {
	var gotUA, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := &Fetcher{}
	if _, err := f.Fetch(context.Background(), srv.URL, ""); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotUA != DefaultUserAgent {
		t.Errorf("User-Agent = %q; want default %q", gotUA, DefaultUserAgent)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q; want application/json", gotAccept)
	}

	// A custom UserAgent must override the default.
	f = &Fetcher{UserAgent: "custom-agent/9"}
	if _, err := f.Fetch(context.Background(), srv.URL, ""); err != nil {
		t.Fatalf("Fetch with custom UA: %v", err)
	}
	if gotUA != "custom-agent/9" {
		t.Errorf("User-Agent = %q; want custom-agent/9", gotUA)
	}

	// nil Client → default client with the package timeout.
	c := (&Fetcher{}).client()
	if c.Timeout != DefaultFetchTimeout {
		t.Errorf("default client timeout = %v; want %v", c.Timeout, DefaultFetchTimeout)
	}
}

func TestFetch_SizeCapAndBadStatus(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "body exceeds size cap",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("x", MaxFeedBytes+1)))
			},
		},
		{
			name: "404 not found",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
		},
		{
			name: "500 server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			f := &Fetcher{}
			res, err := f.Fetch(context.Background(), srv.URL, "")
			if err == nil {
				t.Fatalf("Fetch succeeded (NotModified=%v, %d bytes); want error", res.NotModified, len(res.Body))
			}
		})
	}

	// Empty URL is rejected before any request is made.
	if _, err := (&Fetcher{}).Fetch(context.Background(), "", ""); err == nil {
		t.Fatal("Fetch with empty URL succeeded; want error")
	}
}
