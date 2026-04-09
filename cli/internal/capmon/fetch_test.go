package capmon_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestFetchSource_Success(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("provider docs content"))
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	capmon.SetHTTPClientForTest(ts.Client())
	defer capmon.SetHTTPClientForTest(nil)

	entry, err := capmon.FetchSource(context.Background(), cacheDir, "test-provider", "docs", ts.URL+"/docs")
	if err != nil {
		t.Fatalf("FetchSource: %v", err)
	}
	if string(entry.Raw) != "provider docs content" {
		t.Errorf("unexpected content: %q", string(entry.Raw))
	}
	if entry.Meta.ContentHash == "" {
		t.Error("ContentHash not set")
	}
	if !capmon.IsCached(cacheDir, "test-provider", "docs") {
		t.Error("cache entry not written")
	}
}

func TestFetchSource_RetryOnTransient(t *testing.T) {
	attempts := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("success after retries"))
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	entry, err := capmon.FetchSource(context.Background(), cacheDir, "retry-provider", "docs", ts.URL+"/docs")
	if err != nil {
		t.Fatalf("FetchSource: %v", err)
	}
	if attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempts)
	}
	if string(entry.Raw) != "success after retries" {
		t.Errorf("unexpected content: %q", string(entry.Raw))
	}
}

func TestFetchSource_HashUnchanged(t *testing.T) {
	content := []byte("stable content")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()

	cacheDir := t.TempDir()
	e1, _ := capmon.FetchSource(context.Background(), cacheDir, "stable", "src", ts.URL)
	e2, err := capmon.FetchSource(context.Background(), cacheDir, "stable", "src", ts.URL)
	if err != nil {
		t.Fatalf("second FetchSource: %v", err)
	}
	if e1.Meta.ContentHash != e2.Meta.ContentHash {
		t.Error("hash should be identical for unchanged content")
	}
	if !e2.Meta.Cached {
		t.Error("second fetch should be marked as cached")
	}
}

func TestValidateSourceURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{"valid https", "https://docs.anthropic.com/llms-full.txt", false, ""},
		{"http rejected", "http://example.com", true, "only https scheme allowed"},
		{"raw IPv4", "https://127.0.0.1/path", true, "raw IP literal not allowed"},
		{"raw IPv6", "https://[::1]/path", true, "raw IP literal not allowed"},
		{"loopback hostname", "https://localhost/path", true, "reserved IP"},
		{"link-local", "https://169.254.169.254/latest/meta-data", true, "raw IP literal not allowed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := capmon.ValidateSourceURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSourceURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
