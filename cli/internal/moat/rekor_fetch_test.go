package moat

// Tests for FetchRekorEntry — the per-item Rekor bundle fetcher (bead
// syllago-ndj5v). This is the missing piece of the SIGNED/DUAL-ATTESTED
// install path: VerifyAttestationItem already implements the full chain
// (ECDSA → SET → inclusion proof → Fulcio identity → repo-ID binding) but
// needs the raw Rekor JSON bytes for a given logIndex. SyncResult does
// NOT carry per-item bundles, so the install flow has to fetch them
// itself.
//
// Coverage focuses on:
//
//   - Round-trip: server returns canned bytes, FetchRekorEntry returns
//     them verbatim. The exact bytes matter — VerifyAttestationItem hashes
//     and re-parses them, so any re-marshaling would invalidate the
//     signature.
//   - Query param construction: logIndex must be on the query string
//     because the Rekor lookup API rejects body-style requests.
//   - User-Agent: identifies syllago to Rekor for ratelimit accounting.
//   - Status / size / negative-input guards: defense in depth.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withRekorBase points the fetcher at a test server for the duration of
// the test, restoring the production URL on cleanup. Mutates a package
// global, so tests using this helper MUST NOT call t.Parallel() — running
// concurrently would clobber each other's server URLs.
func withRekorBase(t *testing.T, base string) {
	t.Helper()
	orig := rekorBaseURL
	rekorBaseURL = base
	t.Cleanup(func() { rekorBaseURL = orig })
}

func TestFetchRekorEntry_RoundTripBytes(t *testing.T) {
	want := []byte(`{"abc123":{"body":"...","logIndex":42}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(want)
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	got, err := FetchRekorEntry(context.Background(), 42)
	if err != nil {
		t.Fatalf("FetchRekorEntry: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("body mismatch:\n got=%q\nwant=%q", got, want)
	}
}

func TestFetchRekorEntry_PutsLogIndexOnQuery(t *testing.T) {
	var seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	if _, err := FetchRekorEntry(context.Background(), 1336116369); err != nil {
		t.Fatalf("FetchRekorEntry: %v", err)
	}
	if !strings.Contains(seenQuery, "logIndex=1336116369") {
		t.Errorf("expected logIndex on query, got %q", seenQuery)
	}
}

func TestFetchRekorEntry_SendsUserAgent(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	if _, err := FetchRekorEntry(context.Background(), 1); err != nil {
		t.Fatalf("FetchRekorEntry: %v", err)
	}
	if seenUA != DefaultUserAgent {
		t.Errorf("UA = %q, want %q", seenUA, DefaultUserAgent)
	}
}

func TestFetchRekorEntry_NonOKIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	_, err := FetchRekorEntry(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention status code", err.Error())
	}
}

func TestFetchRekorEntry_OversizeIsError(t *testing.T) {
	// Hand a body larger than the size cap; FetchRekorEntry must reject
	// before reading further (or after detecting overflow on read).
	big := strings.Repeat("a", maxRekorBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, big)
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	_, err := FetchRekorEntry(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected oversize error, got nil")
	}
	if !strings.Contains(err.Error(), "exceed") {
		t.Errorf("error %q does not mention size cap", err.Error())
	}
}

func TestFetchRekorEntry_NegativeLogIndexIsError(t *testing.T) {
	// Reject before any network IO — a negative logIndex is a programmer
	// error from a malformed manifest entry, and surfacing it locally
	// keeps a noisy 4xx out of Rekor's logs.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	_, err := FetchRekorEntry(context.Background(), -1)
	if err == nil {
		t.Fatalf("expected error on negative logIndex, got nil")
	}
	if called {
		t.Errorf("network call happened despite negative logIndex")
	}
}

func TestFetchRekorEntry_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := FetchRekorEntry(ctx, 1)
	if err == nil {
		t.Fatalf("expected context cancel to surface as error")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("error %q does not surface context cancel", err.Error())
	}
}

// TestFetchRekorEntry_BytesAreParseable proves the round-trip output is
// directly consumable by parseRekorEntry — the production caller hands
// these bytes to VerifyAttestationItem, which calls parseRekorEntry
// internally. Keeping this assertion local catches a regression where
// FetchRekorEntry might transform bytes (e.g. trim whitespace) in a way
// that breaks the verifier.
func TestFetchRekorEntry_BytesAreParseable(t *testing.T) {
	// Minimal valid rekorResponse: single-entry map keyed by UUID.
	body := `{
  "abc123": {
    "body": "ZXlKaGNHbFdaWEp6YVc5dUlqb2lNQzR3TGpFaWZRPT0=",
    "integratedTime": 1700000000,
    "logID": "deadbeef",
    "logIndex": 42,
    "verification": {
      "inclusionProof": {
        "checkpoint": "",
        "hashes": [],
        "logIndex": 42,
        "rootHash": "",
        "treeSize": 100
      },
      "signedEntryTimestamp": ""
    }
  }
}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	defer srv.Close()
	withRekorBase(t, srv.URL+"/api/v1/log/entries")

	got, err := FetchRekorEntry(context.Background(), 42)
	if err != nil {
		t.Fatalf("FetchRekorEntry: %v", err)
	}
	entry, err := parseRekorEntry(got)
	if err != nil {
		t.Fatalf("parseRekorEntry on fetched bytes: %v", err)
	}
	if entry.LogIndex != 42 {
		t.Errorf("LogIndex = %d, want 42", entry.LogIndex)
	}
}
