package moat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublisherOwnerRepoFromSourceURI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{"github archive tag", "https://github.com/Acme/syllago-meta-registry/archive/refs/tags/v1.0.tar.gz", "Acme/syllago-meta-registry", false},
		{"github release asset", "https://github.com/Acme/repo/releases/download/v1/asset.tar.gz", "Acme/repo", false},
		{"api github tarball", "https://api.github.com/repos/Acme/repo/tarball/abc123", "Acme/repo", false},
		{"non-https scheme", "git+https://github.com/x/y.git", "", true},
		{"unknown host", "https://gitlab.com/x/y/archive/main.tar.gz", "", true},
		{"github too short", "https://github.com/onlyowner", "", true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := publisherOwnerRepoFromSourceURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got %q", tc.uri, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindPublisherEntry_HappyPath(t *testing.T) {
	t.Parallel()
	hash := "sha256:" + strings.Repeat("ab", 32)
	body := []byte(fmt.Sprintf(`{
		"schema_version": 1,
		"items": [
			{"name": "other", "content_hash": "sha256:%s", "rekor_log_index": 11},
			{"name": "split-rules-llm", "content_hash": %q, "rekor_log_index": 42}
		]
	}`, strings.Repeat("ff", 32), hash))

	idx, err := FindPublisherEntry(body, hash)
	if err != nil {
		t.Fatalf("FindPublisherEntry: %v", err)
	}
	if idx != 42 {
		t.Errorf("got logIndex %d, want 42", idx)
	}
}

func TestFindPublisherEntry_CaseInsensitiveHashMatch(t *testing.T) {
	t.Parallel()
	body := []byte(`{"items":[{"name":"x","content_hash":"sha256:AABB","rekor_log_index":7}]}`)
	idx, err := FindPublisherEntry(body, "sha256:aabb")
	if err != nil {
		t.Fatalf("expected case-insensitive match: %v", err)
	}
	if idx != 7 {
		t.Errorf("got %d, want 7", idx)
	}
}

func TestFindPublisherEntry_NoMatch(t *testing.T) {
	t.Parallel()
	body := []byte(`{"items":[{"name":"other","content_hash":"sha256:dead","rekor_log_index":1}]}`)
	_, err := FindPublisherEntry(body, "sha256:beef")
	if err == nil || !strings.Contains(err.Error(), "no items[]") {
		t.Errorf("expected no-match error, got %v", err)
	}
}

func TestFindPublisherEntry_BadJSON(t *testing.T) {
	t.Parallel()
	_, err := FindPublisherEntry([]byte("not json"), "sha256:any")
	if err == nil || !strings.Contains(err.Error(), "parse json") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestFetchPublisherAttestation_HappyPath(t *testing.T) {
	body := []byte(`{"schema_version":1,"items":[]}`)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	orig := PublisherAttestationBaseURLForTest()
	SetPublisherAttestationBaseURLForTest(srv.URL)
	t.Cleanup(func() { SetPublisherAttestationBaseURLForTest(orig) })

	got, err := FetchPublisherAttestation(context.Background(),
		"https://github.com/Acme/repo/archive/refs/tags/v1.tar.gz")
	if err != nil {
		t.Fatalf("FetchPublisherAttestation: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("body did not round-trip: got %q", got)
	}
	wantPath := "/Acme/repo/moat-attestation/moat-attestation.json"
	if gotPath != wantPath {
		t.Errorf("server saw path %q, want %q", gotPath, wantPath)
	}
}

func TestFetchPublisherAttestation_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	orig := PublisherAttestationBaseURLForTest()
	SetPublisherAttestationBaseURLForTest(srv.URL)
	t.Cleanup(func() { SetPublisherAttestationBaseURLForTest(orig) })

	_, err := FetchPublisherAttestation(context.Background(),
		"https://github.com/Acme/repo/archive/refs/tags/v1.tar.gz")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestFetchPublisherAttestation_RejectsBadSourceURI(t *testing.T) {
	_, err := FetchPublisherAttestation(context.Background(), "git+https://example.com/repo.git")
	if err == nil {
		t.Fatal("expected error for non-https scheme")
	}
}

func TestFetchPublisherAttestation_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		// hang forever; canceled ctx must short-circuit
		select {}
	}))
	t.Cleanup(srv.Close)

	orig := PublisherAttestationBaseURLForTest()
	SetPublisherAttestationBaseURLForTest(srv.URL)
	t.Cleanup(func() { SetPublisherAttestationBaseURLForTest(orig) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchPublisherAttestation(ctx,
		"https://github.com/Acme/repo/archive/refs/tags/v1.tar.gz")
	if err == nil || !errors.Is(err, context.Canceled) {
		// some HTTP transports surface as wrapped error — accept either
		if err == nil || !strings.Contains(err.Error(), "context") {
			t.Errorf("expected context cancellation error, got %v", err)
		}
	}
}
