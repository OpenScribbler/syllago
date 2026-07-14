package capfeed

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// runFixture wires the full snapshot behind httptest: a feed server (index +
// files, with request counting), the attestations API behind the package
// seam, and a Run Options value pointed at temp state files.
type runFixture struct {
	opts          Options
	capDir        string
	indexETag     string
	fileRequests  *atomic.Int64
	attRequests   *atomic.Int64
	indexRequests *atomic.Int64
}

func newRunFixture(t *testing.T, indexBytes []byte) *runFixture {
	t.Helper()
	_, bundleBytes := loadSnapshot(t)

	var fileReqs, attReqs, indexReqs atomic.Int64
	const etag = `"snapshot-etag-1"`

	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if rel == "index.json" {
			indexReqs.Add(1)
			if r.Header.Get("If-None-Match") == etag {
				w.Header().Set("ETag", etag)
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", etag)
			_, _ = w.Write(indexBytes)
			return
		}
		fileReqs.Add(1)
		data, err := os.ReadFile(filepath.Join("testdata", "feedsnapshot", "files", filepath.FromSlash(rel)))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(feed.Close)

	att := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attReqs.Add(1)
		_, _ = w.Write([]byte(`{"attestations": [{"bundle": ` + string(bundleBytes) + `}]}`))
	}))
	t.Cleanup(att.Close)
	restore := SetAttestationsAPIBaseURLForTest(att.URL + "/attestations/")
	t.Cleanup(restore)

	repoRoot := t.TempDir()
	capDir := filepath.Join(repoRoot, "docs", "provider-capabilities")
	if err := os.MkdirAll(capDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var idxMeta struct {
		GeneratedAt time.Time `json:"generated_at"`
	}
	if err := json.Unmarshal(indexBytes, &idxMeta); err != nil {
		t.Fatalf("decoding index for fixture clock: %v", err)
	}

	stateDir := t.TempDir()
	return &runFixture{
		opts: Options{
			FeedURL:         feed.URL + "/index.json",
			RepoRoot:        repoRoot,
			ETagFile:        filepath.Join(stateDir, "etag"),
			SummaryFile:     filepath.Join(stateDir, "summary.json"),
			TrustedRootJSON: trustedRootBytes(t),
			Now:             func() time.Time { return idxMeta.GeneratedAt.Add(time.Hour) },
		},
		capDir:        capDir,
		indexETag:     etag,
		fileRequests:  &fileReqs,
		attRequests:   &attReqs,
		indexRequests: &indexReqs,
	}
}

func TestRun_FirstPullMirrorsAndReportsChanged(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	sum, err := Run(context.Background(), fx.opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sum.Changed {
		t.Error("first pull Summary.Changed = false; want true")
	}
	if _, err := os.Stat(filepath.Join(fx.capDir, "capabilities", "claude-code.json")); err != nil {
		t.Errorf("mirror missing capabilities/claude-code.json: %v", err)
	}
}

func TestRun_RevisionMatchShortCircuits(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	// First pull mirrors and writes the marker.
	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	fx.fileRequests.Store(0)

	// Remove the ETag file so the second run gets a full 200 index response
	// and must rely on the data_revision marker comparison, not HTTP 304.
	if err := os.Remove(fx.opts.ETagFile); err != nil {
		t.Fatal(err)
	}

	sum, err := Run(context.Background(), fx.opts)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if sum.Changed {
		t.Error("revision-match run Summary.Changed = true; want false")
	}
	if n := fx.fileRequests.Load(); n != 0 {
		t.Errorf("revision-match run made %d per-file requests; want 0", n)
	}
	if sum.DataRevision == "" {
		t.Error("short-circuit summary lost the data_revision")
	}
}

func TestRun_NotModified304ShortCircuits(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	// First pull persists the server ETag.
	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	fx.attRequests.Store(0)
	fx.fileRequests.Store(0)

	sum, err := Run(context.Background(), fx.opts)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if sum.Changed {
		t.Error("304 run Summary.Changed = true; want false")
	}
	if n := fx.attRequests.Load(); n != 0 {
		t.Errorf("304 run fetched %d attestations; want 0 (no verification work on an unchanged feed)", n)
	}
	if n := fx.fileRequests.Load(); n != 0 {
		t.Errorf("304 run made %d per-file requests; want 0", n)
	}
}

func TestRun_ETagRoundTrip(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	persisted, err := os.ReadFile(fx.opts.ETagFile)
	if err != nil {
		t.Fatalf("ETag file not persisted: %v", err)
	}
	if strings.TrimSpace(string(persisted)) != fx.indexETag {
		t.Errorf("persisted ETag = %q; want server ETag %q", persisted, fx.indexETag)
	}

	// Second run must send it back: the fixture's index handler answers 304
	// only when If-None-Match matches, and the 304 short-circuit is
	// observable via zero attestation requests.
	fx.attRequests.Store(0)
	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if n := fx.attRequests.Load(); n != 0 {
		t.Error("second run did not send If-None-Match (no 304 short-circuit happened)")
	}
}

func TestRun_VerifyPrecedesMarkerCompare(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	// Mirror once so the marker matches the snapshot's data_revision.
	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := os.Remove(fx.opts.ETagFile); err != nil {
		t.Fatal(err)
	}

	// Tamper INSIDE a JSON string value (one sha256 hex digit) so the index
	// still parses and its data_revision still matches the marker — only
	// provenance verification can reject it. This proves verification runs
	// BEFORE the marker comparison is trusted.
	tampered := tamperOneSHA256(t, indexBytes)
	idxCheck, err := ParseIndex(tampered)
	if err != nil {
		t.Fatalf("tampered index must remain parseable (the point is a signature failure, not a parse failure): %v", err)
	}
	origIdx, err := ParseIndex(indexBytes)
	if err != nil {
		t.Fatal(err)
	}
	if idxCheck.DataRevision != origIdx.DataRevision {
		t.Fatal("tamper changed data_revision; the marker short-circuit would not be reached")
	}

	fx2 := newRunFixture(t, tampered)
	fx2.opts.RepoRoot = fx.opts.RepoRoot // same repo: marker matches

	if _, err := Run(context.Background(), fx2.opts); err == nil {
		t.Fatal("Run verified a tampered index whose revision matches the marker; want error before the short-circuit")
	}
}

// tamperOneSHA256 flips one hex digit inside the first file sha256 string in
// the raw index JSON, keeping the document valid.
func tamperOneSHA256(t *testing.T, indexBytes []byte) []byte {
	t.Helper()
	var meta struct {
		DataRevision string `json:"data_revision"`
		Files        map[string]struct {
			SHA256 string `json:"sha256"`
		} `json:"files"`
	}
	if err := json.Unmarshal(indexBytes, &meta); err != nil {
		t.Fatal(err)
	}
	for _, f := range meta.Files {
		// data_revision is the sha256 of one of the files, and it appears in
		// the raw JSON before the files map — flipping that file's digest via
		// first-occurrence Replace would tamper data_revision instead.
		if f.SHA256 == meta.DataRevision {
			continue
		}
		old := []byte(`"` + f.SHA256 + `"`)
		flipped := []byte(f.SHA256)
		if flipped[0] == 'a' {
			flipped[0] = 'b'
		} else {
			flipped[0] = 'a'
		}
		tampered := bytes.Replace(indexBytes, old, []byte(`"`+string(flipped)+`"`), 1)
		if !bytes.Equal(tampered, indexBytes) {
			return tampered
		}
	}
	t.Fatal("no sha256 value found to tamper")
	return nil
}

func TestRun_SummaryJSON(t *testing.T) {
	indexBytes, _ := loadSnapshot(t)
	fx := newRunFixture(t, indexBytes)

	if _, err := Run(context.Background(), fx.opts); err != nil {
		t.Fatalf("Run: %v", err)
	}
	raw, err := os.ReadFile(fx.opts.SummaryFile)
	if err != nil {
		t.Fatalf("summary file: %v", err)
	}
	var got struct {
		Changed          bool     `json:"changed"`
		DataRevision     string   `json:"data_revision"`
		GeneratedAt      string   `json:"generated_at"`
		ChangedProviders []string `json:"changed_providers"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("summary is not valid JSON: %v", err)
	}
	if !got.Changed {
		t.Error("summary changed = false on a first pull; want true")
	}
	if got.DataRevision == "" || got.GeneratedAt == "" {
		t.Errorf("summary missing identity fields: %+v", got)
	}
	if len(got.ChangedProviders) == 0 {
		t.Error("summary changed_providers empty on a first pull; want every provider slug")
	}
}
