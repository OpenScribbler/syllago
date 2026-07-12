package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// serveSnapshotFeed serves the full captured snapshot: the index at
// /index.json and every attested file from testdata/feedsnapshot/files/.
// corrupt, when non-empty, names one feed-relative path whose bytes are
// served with the first byte flipped.
func serveSnapshotFeed(t *testing.T, indexBytes []byte, corrupt string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if rel == "index.json" {
			_, _ = w.Write(indexBytes)
			return
		}
		data, err := os.ReadFile(filepath.Join(snapshotDir, "files", filepath.FromSlash(rel)))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if rel == corrupt {
			data = append([]byte(nil), data...)
			data[0] ^= 0x01
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestMain_PullWritesVerifiedMirror(t *testing.T) {
	indexBytes, revision, generatedAt := readSnapshot(t)
	serveAttestations(t)
	pinClock(t, generatedAt)

	repoRoot := t.TempDir()
	capDir := filepath.Join(repoRoot, "docs", "provider-capabilities")
	if err := os.MkdirAll(capDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Pre-existing directory state: a stale YAML baseline (must be swept)
	// and a keep-list README (must survive).
	if err := os.WriteFile(filepath.Join(capDir, "claude-code.yaml"), []byte("stale: yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capDir, "README.md"), []byte("# keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	feed := serveSnapshotFeed(t, indexBytes, "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-feed-url", feed.URL + "/index.json", "-repo-root", repoRoot}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("full pull exited %d; stderr: %s", code, stderr.String())
	}

	// A provider Capability Document is mirrored byte-for-byte.
	want, err := os.ReadFile(filepath.Join(snapshotDir, "files", "capabilities", "claude-code.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(capDir, "capabilities", "claude-code.json"))
	if err != nil {
		t.Fatalf("mirrored claude-code.json: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("mirrored capabilities/claude-code.json is not byte-identical to the feed copy")
	}

	// Marker records the verified revision; sweep + keep-list applied;
	// advisories.json is excluded from the mirror by design.
	marker, err := os.ReadFile(filepath.Join(capDir, "provenance.json"))
	if err != nil {
		t.Fatalf("provenance.json: %v", err)
	}
	if !strings.Contains(string(marker), revision) {
		t.Errorf("provenance.json %q does not record data_revision %q", marker, revision)
	}
	if _, err := os.Stat(filepath.Join(capDir, "claude-code.yaml")); !os.IsNotExist(err) {
		t.Error("stale claude-code.yaml survived the sweep")
	}
	if _, err := os.Stat(filepath.Join(capDir, "README.md")); err != nil {
		t.Error("keep-list README.md was swept away")
	}
	if _, err := os.Stat(filepath.Join(capDir, "advisories.json")); !os.IsNotExist(err) {
		t.Error("advisories.json was mirrored; it is out of scope (not mirrored, not acted on)")
	}

	// Second run against a feed serving one corrupted file: exits non-zero
	// and the mirrored tree is byte-for-byte unchanged. The marker is
	// removed first so the data_revision short-circuit doesn't skip the
	// re-fetch — this run must reach the per-file hash gate.
	if err := os.Remove(filepath.Join(capDir, "provenance.json")); err != nil {
		t.Fatal(err)
	}
	before := treeDigest(t, capDir)
	corruptFeed := serveSnapshotFeed(t, indexBytes, "capabilities/claude-code.json")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"-feed-url", corruptFeed.URL + "/index.json", "-repo-root", repoRoot}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("pull with a corrupted feed file exited 0; want non-zero")
	}
	if after := treeDigest(t, capDir); after != before {
		t.Error("mirror tree changed during a failed pull; fail-closed means nothing is written")
	}
}

func TestMain_SecondRunIsNoOp(t *testing.T) {
	indexBytes, _, generatedAt := readSnapshot(t)
	serveAttestations(t)
	pinClock(t, generatedAt)

	repoRoot := t.TempDir()
	stateDir := t.TempDir()
	etagFile := filepath.Join(stateDir, "etag")
	summaryFile := filepath.Join(stateDir, "summary.json")
	if err := os.MkdirAll(filepath.Join(repoRoot, "docs", "provider-capabilities"), 0o755); err != nil {
		t.Fatal(err)
	}
	feed := serveSnapshotFeed(t, indexBytes, "")
	pullArgs := []string{
		"-feed-url", feed.URL + "/index.json",
		"-repo-root", repoRoot,
		"-etag-file", etagFile,
		"-summary-file", summaryFile,
	}

	var stdout, stderr bytes.Buffer
	if code := run(pullArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("first pull exited %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "changed: true") {
		t.Errorf("first pull stdout %q does not report changed: true", stdout.String())
	}

	capDir := filepath.Join(repoRoot, "docs", "provider-capabilities")
	before := treeDigest(t, capDir)

	stdout.Reset()
	stderr.Reset()
	if code := run(pullArgs, &stdout, &stderr); code != 0 {
		t.Fatalf("second pull exited %d; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "changed: false") {
		t.Errorf("second pull stdout %q does not report changed: false", stdout.String())
	}
	if after := treeDigest(t, capDir); after != before {
		t.Error("second pull against an unchanged feed modified the mirror")
	}

	// The summary file reflects the no-op for the workflow to consume.
	raw, err := os.ReadFile(summaryFile)
	if err != nil {
		t.Fatalf("summary file: %v", err)
	}
	var sum struct {
		Changed bool `json:"changed"`
	}
	if err := json.Unmarshal(raw, &sum); err != nil {
		t.Fatalf("summary JSON: %v", err)
	}
	if sum.Changed {
		t.Error("summary changed = true after a no-op run; want false")
	}
}

// treeDigest returns a digest of every file path + content under dir.
func treeDigest(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		h.Write([]byte(rel))
		h.Write(data)
		return nil
	})
	if err != nil {
		t.Fatalf("digesting %s: %v", dir, err)
	}
	return hex.EncodeToString(h.Sum(nil))
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
