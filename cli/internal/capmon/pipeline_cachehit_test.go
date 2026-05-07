package capmon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestRunStage1Fetch_CacheHitCounter verifies that ProviderStatus.SourcesCacheHit
// is incremented when a source's content hash is unchanged from the previous fetch.
// Uses SetValidateURLForTest to bypass SSRF validation for the httptest server URL.
func TestRunStage1Fetch_CacheHitCounter(t *testing.T) {
	content := []byte("deterministic provider docs content")
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer ts.Close()
	SetHTTPClientForTest(ts.Client())
	defer SetHTTPClientForTest(nil)
	SetValidateURLForTest(func(string) error { return nil })
	defer SetValidateURLForTest(nil)

	cacheDir := t.TempDir()
	srcDir := t.TempDir()

	manifestYAML := fmt.Sprintf(`schema_version: "1"
slug: test-cachehit
content_types:
  rules:
    sources:
      - url: "%s/docs"
        format: html
`, ts.URL)
	if err := os.WriteFile(srcDir+"/test-cachehit.yaml", []byte(manifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := PipelineOptions{
		CacheRoot:          cacheDir,
		SourceManifestsDir: srcDir,
	}

	// First fetch: content not in cache, SourcesCacheHit should be 0.
	m1 := &RunManifest{RunID: "run-1", Providers: make(map[string]ProviderStatus)}
	if err := runStage1Fetch(context.Background(), opts, m1); err != nil {
		t.Fatalf("first runStage1Fetch: %v", err)
	}
	s1 := m1.Providers["test-cachehit"]
	if len(s1.Errors) > 0 {
		t.Fatalf("unexpected errors on first fetch: %v", s1.Errors)
	}
	if s1.SourcesFetched != 1 {
		t.Errorf("SourcesFetched = %d, want 1", s1.SourcesFetched)
	}
	if s1.SourcesCacheHit != 0 {
		t.Errorf("SourcesCacheHit = %d, want 0 on first fetch", s1.SourcesCacheHit)
	}

	// Second fetch: same content, hash matches cached entry, SourcesCacheHit should be 1.
	m2 := &RunManifest{RunID: "run-2", Providers: make(map[string]ProviderStatus)}
	if err := runStage1Fetch(context.Background(), opts, m2); err != nil {
		t.Fatalf("second runStage1Fetch: %v", err)
	}
	s2 := m2.Providers["test-cachehit"]
	if len(s2.Errors) > 0 {
		t.Errorf("unexpected errors on second fetch: %v", s2.Errors)
	}
	if s2.SourcesFetched != 1 {
		t.Errorf("SourcesFetched = %d, want 1 (cache hit still counts as fetched)", s2.SourcesFetched)
	}
	if s2.SourcesCacheHit != 1 {
		t.Errorf("SourcesCacheHit = %d, want 1 on repeated fetch of same content", s2.SourcesCacheHit)
	}
}

// TestRunStage1Fetch_CacheHitZeroOnChanged verifies that SourcesCacheHit remains
// 0 when content hash changes between fetches (fresh content, not a cache hit).
func TestRunStage1Fetch_CacheHitZeroOnChanged(t *testing.T) {
	requestCount := 0
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		fmt.Fprintf(w, "content version %d — unique hash per request", requestCount)
	}))
	defer ts.Close()
	SetHTTPClientForTest(ts.Client())
	defer SetHTTPClientForTest(nil)
	SetValidateURLForTest(func(string) error { return nil })
	defer SetValidateURLForTest(nil)

	cacheDir := t.TempDir()
	srcDir := t.TempDir()

	manifestYAML := fmt.Sprintf(`schema_version: "1"
slug: test-changed
content_types:
  rules:
    sources:
      - url: "%s/docs"
        format: html
`, ts.URL)
	if err := os.WriteFile(srcDir+"/test-changed.yaml", []byte(manifestYAML), 0644); err != nil {
		t.Fatal(err)
	}

	opts := PipelineOptions{
		CacheRoot:          cacheDir,
		SourceManifestsDir: srcDir,
	}

	m1 := &RunManifest{RunID: "run-1", Providers: make(map[string]ProviderStatus)}
	if err := runStage1Fetch(context.Background(), opts, m1); err != nil {
		t.Fatal(err)
	}

	// Second fetch: different content → SourcesCacheHit must be 0.
	m2 := &RunManifest{RunID: "run-2", Providers: make(map[string]ProviderStatus)}
	if err := runStage1Fetch(context.Background(), opts, m2); err != nil {
		t.Fatal(err)
	}
	s2 := m2.Providers["test-changed"]
	if s2.SourcesCacheHit != 0 {
		t.Errorf("SourcesCacheHit = %d, want 0 when content changes between fetches", s2.SourcesCacheHit)
	}
	if s2.SourcesFetched != 1 {
		t.Errorf("SourcesFetched = %d, want 1", s2.SourcesFetched)
	}
}

// TestProviderStatus_JSONRoundtrip_SourcesCacheHit verifies that the new
// SourcesCacheHit field survives JSON marshal/unmarshal and that zero value
// is omitted from the output (omitempty — preserves backward compatibility
// with existing last-run.json files that predate this field).
func TestProviderStatus_JSONRoundtrip_SourcesCacheHit(t *testing.T) {
	t.Run("non-zero value preserved", func(t *testing.T) {
		ps := ProviderStatus{
			Slug:            "claude-code",
			SourcesFetched:  3,
			SourcesCacheHit: 2,
		}
		data, err := json.Marshal(ps)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !strings.Contains(string(data), `"sources_cache_hit"`) {
			t.Errorf("JSON missing sources_cache_hit key:\n%s", data)
		}
		var round ProviderStatus
		if err := json.Unmarshal(data, &round); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if round.SourcesCacheHit != 2 {
			t.Errorf("SourcesCacheHit = %d, want 2 after roundtrip", round.SourcesCacheHit)
		}
	})

	t.Run("zero value omitted", func(t *testing.T) {
		ps := ProviderStatus{
			Slug:           "cursor",
			SourcesFetched: 1,
		}
		data, err := json.Marshal(ps)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if strings.Contains(string(data), "sources_cache_hit") {
			t.Errorf("expected sources_cache_hit to be omitted for zero value; got:\n%s", data)
		}
	})
}
