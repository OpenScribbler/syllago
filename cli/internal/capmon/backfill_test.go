package capmon_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// stubFetcher records calls and returns canned bodies keyed by URI. Used to
// test BackfillFormatDoc without hitting live HTTP or chromedp.
type stubFetcher struct {
	responses map[string][]byte
	calls     []stubFetcherCall
}

type stubFetcherCall struct {
	URI         string
	FetchMethod string
}

func (s *stubFetcher) Fetch(_ context.Context, uri, fetchMethod string) ([]byte, error) {
	s.calls = append(s.calls, stubFetcherCall{URI: uri, FetchMethod: fetchMethod})
	body, ok := s.responses[uri]
	if !ok {
		return nil, fmt.Errorf("no stub response for %s", uri)
	}
	return body, nil
}

// TestBackfillFormatDoc_FillsMissingHash is the B1 RED test for syllago-73k9a.
// It asserts that BackfillFormatDoc:
//  1. Populates content_hash for sources where it was empty.
//  2. Skips sources whose content_hash is already populated (Force=false).
//  3. Updates fetched_at on modified sources + top-level last_fetched_at.
//  4. Preserves YAML comments and key order via yaml.Node in-place editing.
func TestBackfillFormatDoc_FillsMissingHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")

	initial := `# Header comment preserved across backfill
provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/prefilled.md"
        type: documentation
        fetch_method: md_url
        content_hash: "sha256:prefilled_hash_should_not_change"
        fetched_at: "2026-04-11T00:00:00Z"
      - uri: "https://example.com/missing.md"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: ""
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions: []
`
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	missingBody := []byte("fresh content for missing source")
	fetcher := &stubFetcher{
		responses: map[string][]byte{
			"https://example.com/missing.md": missingBody,
			// No stub for prefilled.md — if Force=false incorrectly re-fetches it,
			// the stub returns an error and the test fails.
		},
	}

	result, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{Force: false})
	if err != nil {
		t.Fatalf("BackfillFormatDoc: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1", result.Updated)
	}

	// Exactly one fetch call, for the missing URI.
	if len(fetcher.calls) != 1 {
		t.Fatalf("fetcher calls = %d (%+v), want 1", len(fetcher.calls), fetcher.calls)
	}
	if fetcher.calls[0].URI != "https://example.com/missing.md" {
		t.Errorf("fetcher call URI = %q, want https://example.com/missing.md", fetcher.calls[0].URI)
	}

	// Reload the document and assert field-level state.
	doc, err := capmon.LoadFormatDoc(path)
	if err != nil {
		t.Fatalf("LoadFormatDoc: %v", err)
	}
	skills, ok := doc.ContentTypes["skills"]
	if !ok {
		t.Fatalf("expected skills content_type in reloaded doc")
	}
	if len(skills.Sources) != 2 {
		t.Fatalf("sources len = %d, want 2", len(skills.Sources))
	}

	// Prefilled source is untouched.
	if skills.Sources[0].ContentHash != "sha256:prefilled_hash_should_not_change" {
		t.Errorf("prefilled ContentHash = %q, want unchanged", skills.Sources[0].ContentHash)
	}
	if skills.Sources[0].FetchedAt != "2026-04-11T00:00:00Z" {
		t.Errorf("prefilled FetchedAt = %q, want unchanged", skills.Sources[0].FetchedAt)
	}

	// Missing source now populated.
	expectedHash := capmon.SHA256Hex(missingBody)
	if skills.Sources[1].ContentHash != expectedHash {
		t.Errorf("backfilled ContentHash = %q, want %q", skills.Sources[1].ContentHash, expectedHash)
	}
	if skills.Sources[1].FetchedAt == "" {
		t.Error("backfilled FetchedAt is empty, want a recent RFC3339 timestamp")
	}
	if skills.Sources[1].FetchedAt == "2026-04-11T00:00:00Z" {
		t.Error("backfilled FetchedAt was not updated from the initial fixture value")
	}

	// Top-level last_fetched_at updated because at least one source was backfilled.
	if doc.LastFetchedAt == "2026-04-11T00:00:00Z" {
		t.Error("top-level LastFetchedAt was not updated after backfill")
	}

	// yaml.Node preservation: header comment survives the round-trip.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back file: %v", err)
	}
	if !strings.Contains(string(data), "# Header comment preserved across backfill") {
		t.Error("expected header comment to survive backfill; it was stripped")
	}
}

// writeBackfillFixture writes a minimal FormatDoc with a single source and
// returns the file path. The source's content_hash and fetch_method are
// configurable per test.
func writeBackfillFixture(t *testing.T, dir, uri, contentHash, fetchMethod string) string {
	t.Helper()
	path := filepath.Join(dir, "test-provider.yaml")
	content := fmt.Sprintf(`provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: %q
        type: documentation
        fetch_method: %s
        content_hash: %q
        fetched_at: "2026-04-11T00:00:00Z"
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions: []
`, uri, fetchMethod, contentHash)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

// TestBackfillFormatDoc_ForceOverwrites asserts that Force=true re-fetches and
// overwrites an already-populated content_hash.
func TestBackfillFormatDoc_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	uri := "https://example.com/src.md"
	path := writeBackfillFixture(t, dir, uri, "sha256:old_existing_hash", "md_url")

	newBody := []byte("refreshed body")
	fetcher := &stubFetcher{responses: map[string][]byte{uri: newBody}}

	result, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{Force: true})
	if err != nil {
		t.Fatalf("BackfillFormatDoc: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1", result.Updated)
	}
	if len(fetcher.calls) != 1 {
		t.Fatalf("fetcher calls = %d, want 1", len(fetcher.calls))
	}
	doc, err := capmon.LoadFormatDoc(path)
	if err != nil {
		t.Fatalf("LoadFormatDoc: %v", err)
	}
	got := doc.ContentTypes["skills"].Sources[0].ContentHash
	want := capmon.SHA256Hex(newBody)
	if got != want {
		t.Errorf("ContentHash after Force = %q, want %q", got, want)
	}
}

// TestBackfillFormatDoc_ChromedpRouting asserts that fetch_method: chromedp is
// passed through to the SourceFetcher, so production callers can route it to
// FetchChromedp.
func TestBackfillFormatDoc_ChromedpRouting(t *testing.T) {
	dir := t.TempDir()
	uri := "https://example.com/spa.html"
	path := writeBackfillFixture(t, dir, uri, "", "chromedp")

	fetcher := &stubFetcher{responses: map[string][]byte{uri: []byte("rendered DOM")}}

	_, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{})
	if err != nil {
		t.Fatalf("BackfillFormatDoc: %v", err)
	}
	if len(fetcher.calls) != 1 {
		t.Fatalf("fetcher calls = %d, want 1", len(fetcher.calls))
	}
	if fetcher.calls[0].FetchMethod != "chromedp" {
		t.Errorf("FetchMethod passed to fetcher = %q, want %q", fetcher.calls[0].FetchMethod, "chromedp")
	}
}

// TestBackfillFormatDoc_Idempotent asserts that running Backfill twice without
// Force produces no additional fetches and no file changes on the second run.
func TestBackfillFormatDoc_Idempotent(t *testing.T) {
	dir := t.TempDir()
	uri := "https://example.com/src.md"
	path := writeBackfillFixture(t, dir, uri, "", "md_url")

	fetcher := &stubFetcher{responses: map[string][]byte{uri: []byte("body content")}}
	if _, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{}); err != nil {
		t.Fatalf("first BackfillFormatDoc: %v", err)
	}
	afterFirst, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}
	firstCallCount := len(fetcher.calls)

	result2, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{})
	if err != nil {
		t.Fatalf("second BackfillFormatDoc: %v", err)
	}
	if result2.Updated != 0 {
		t.Errorf("second run Updated = %d, want 0", result2.Updated)
	}
	if len(fetcher.calls) != firstCallCount {
		t.Errorf("second run made %d extra fetches, want 0", len(fetcher.calls)-firstCallCount)
	}
	afterSecond, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(afterFirst) != string(afterSecond) {
		t.Error("second idempotent run changed the file")
	}
}

// TestBackfillFormatDoc_PartialFailure asserts that when one source fetch
// fails, BackfillFormatDoc still processes other sources and reports the
// failure via BackfillResult.Errors and a joined error.
func TestBackfillFormatDoc_PartialFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-provider.yaml")
	content := `provider: test-provider
docs_url: "https://example.com/docs"
category: cli
last_fetched_at: "2026-04-11T00:00:00Z"
generation_method: human-edited
content_types:
  skills:
    status: supported
    sources:
      - uri: "https://example.com/ok.md"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: ""
      - uri: "https://example.com/fail.md"
        type: documentation
        fetch_method: md_url
        content_hash: ""
        fetched_at: ""
    canonical_mappings:
      display_name:
        supported: true
        mechanism: "yaml key: name"
        confidence: confirmed
    provider_extensions: []
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	fetcher := &stubFetcher{
		responses: map[string][]byte{
			"https://example.com/ok.md": []byte("good body"),
			// fail.md is absent — stub returns error.
		},
	}

	result, err := capmon.BackfillFormatDoc(context.Background(), path, fetcher, capmon.BackfillOptions{})
	if err == nil {
		t.Fatal("expected aggregate error from partial failure, got nil")
	}
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1 (ok.md succeeded)", result.Updated)
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors = %d, want 1 (fail.md)", len(result.Errors))
	}

	doc, lerr := capmon.LoadFormatDoc(path)
	if lerr != nil {
		t.Fatalf("LoadFormatDoc: %v", lerr)
	}
	srcs := doc.ContentTypes["skills"].Sources
	if srcs[0].ContentHash == "" {
		t.Error("ok.md ContentHash is empty; expected a successful backfill")
	}
	if srcs[1].ContentHash != "" {
		t.Errorf("fail.md ContentHash = %q, want empty (fetch failed)", srcs[1].ContentHash)
	}
}
