package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capfeed"
)

// snapshotDir is the captured live feed snapshot shared with the capfeed
// package tests: real index bytes + their real attestation bundle.
var snapshotDir = filepath.Join("..", "..", "internal", "capfeed", "testdata", "feedsnapshot")

func readSnapshot(t *testing.T) (indexBytes []byte, revision string, generatedAt time.Time) {
	t.Helper()
	indexBytes, err := os.ReadFile(filepath.Join(snapshotDir, "index.json"))
	if err != nil {
		t.Fatalf("reading snapshot index: %v", err)
	}
	var idx struct {
		DataRevision string `json:"data_revision"`
		GeneratedAt  string `json:"generated_at"`
	}
	if err := json.Unmarshal(indexBytes, &idx); err != nil {
		t.Fatalf("decoding snapshot index: %v", err)
	}
	generatedAt, err = time.Parse(time.RFC3339, idx.GeneratedAt)
	if err != nil {
		t.Fatalf("parsing snapshot generated_at: %v", err)
	}
	return indexBytes, idx.DataRevision, generatedAt
}

// serveAttestations returns an httptest server that answers the GitHub
// attestations API with the recorded snapshot bundle, and points the capfeed
// base-URL seam at it for the duration of the test.
func serveAttestations(t *testing.T) {
	t.Helper()
	bundleBytes, err := os.ReadFile(filepath.Join(snapshotDir, "attestation-bundle.json"))
	if err != nil {
		t.Fatalf("reading snapshot bundle: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"attestations": [{"bundle": ` + string(bundleBytes) + `}]}`))
	}))
	t.Cleanup(srv.Close)
	restore := capfeed.SetAttestationsAPIBaseURLForTest(srv.URL + "/attestations/")
	t.Cleanup(restore)
}

// pinClock fixes the tool's clock just after the snapshot's generated_at so
// the freshness gate sees a live feed no matter when the test actually runs.
func pinClock(t *testing.T, generatedAt time.Time) {
	t.Helper()
	prev := nowFunc
	nowFunc = func() time.Time { return generatedAt.Add(time.Hour) }
	t.Cleanup(func() { nowFunc = prev })
}

func TestMain_CheckPrintsRevision(t *testing.T) {
	indexBytes, revision, generatedAt := readSnapshot(t)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(indexBytes)
	}))
	defer feed.Close()
	serveAttestations(t)
	pinClock(t, generatedAt)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-check", "-feed-url", feed.URL}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run -check exited %d; stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, revision) {
		t.Errorf("stdout %q does not contain the data_revision %q", out, revision)
	}
	if !strings.Contains(out, generatedAt.UTC().Format("2006-01-02T15:04:05Z")) {
		t.Errorf("stdout %q does not contain the generated_at timestamp", out)
	}
}

func TestMain_CheckFailsClosedOnTamper(t *testing.T) {
	indexBytes, revision, generatedAt := readSnapshot(t)
	tampered := append([]byte(nil), indexBytes...)
	tampered[len(tampered)/2] ^= 0x01

	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tampered)
	}))
	defer feed.Close()
	serveAttestations(t)
	pinClock(t, generatedAt)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-check", "-feed-url", feed.URL}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run -check exited 0 on a tampered index; want non-zero. stdout: %s", stdout.String())
	}
	// Nothing from the unverified feed may be printed as trusted output.
	if strings.Contains(stdout.String(), revision) {
		t.Errorf("stdout %q leaked the data_revision of an unverified index", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Error("stderr is empty; want a verification error message")
	}
}

func TestMain_CheckFailsClosedOnStaleFeed(t *testing.T) {
	indexBytes, _, generatedAt := readSnapshot(t)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(indexBytes)
	}))
	defer feed.Close()
	serveAttestations(t)

	// Clock pinned 49h after generated_at: past the feed's 48h heartbeat.
	prev := nowFunc
	nowFunc = func() time.Time { return generatedAt.Add(49 * time.Hour) }
	t.Cleanup(func() { nowFunc = prev })

	var stdout, stderr bytes.Buffer
	code := run([]string{"-check", "-feed-url", feed.URL}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("run -check exited 0 on a stale feed; want non-zero. stdout: %s", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Error("stderr is empty; want a staleness error message")
	}
}

func TestMain_CheckFailsOnMalformedIndex(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "malformed JSON body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`<html>not json</html>`))
			},
		},
		{
			name: "missing required fields",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{"cadence": "daily"}`))
			},
		},
		{
			name: "http error status",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			var stdout, stderr bytes.Buffer
			code := run([]string{"-check", "-feed-url", srv.URL}, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("run -check exited 0 with stdout %q; want non-zero", stdout.String())
			}
			if stderr.Len() == 0 {
				t.Error("stderr is empty; want an error message")
			}
		})
	}
}
