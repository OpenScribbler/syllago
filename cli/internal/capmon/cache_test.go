package capmon_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestWriteAndReadCacheEntry(t *testing.T) {
	dir := t.TempDir()
	content := []byte("provider doc content")
	hash := sha256Hash(content)

	entry := capmon.CacheEntry{
		Provider: "claude-code",
		SourceID: "hooks-docs",
		Raw:      content,
		Meta: capmon.CacheMeta{
			FetchedAt:   time.Now().UTC(),
			ContentHash: hash,
			FetchStatus: "ok",
			FetchMethod: "http",
		},
	}

	if err := capmon.WriteCacheEntry(dir, entry); err != nil {
		t.Fatalf("WriteCacheEntry: %v", err)
	}

	got, err := capmon.ReadCacheEntry(dir, "claude-code", "hooks-docs")
	if err != nil {
		t.Fatalf("ReadCacheEntry: %v", err)
	}
	if string(got.Raw) != string(content) {
		t.Errorf("Raw content mismatch")
	}
	if got.Meta.ContentHash != hash {
		t.Errorf("Hash mismatch: got %q, want %q", got.Meta.ContentHash, hash)
	}
}

func TestIsCached_False(t *testing.T) {
	dir := t.TempDir()
	if capmon.IsCached(dir, "no-provider", "no-source") {
		t.Error("expected IsCached to return false for non-existent entry")
	}
}

func TestAgeBasedEvict(t *testing.T) {
	dir := t.TempDir()
	content := []byte("old content")
	hash := sha256Hash(content)

	old := capmon.CacheEntry{
		Provider: "windsurf",
		SourceID: "llms-full",
		Raw:      content,
		Meta: capmon.CacheMeta{
			FetchedAt:   time.Now().UTC().Add(-31 * 24 * time.Hour), // 31 days old
			ContentHash: hash,
			FetchStatus: "ok",
		},
	}
	if err := capmon.WriteCacheEntry(dir, old); err != nil {
		t.Fatalf("WriteCacheEntry: %v", err)
	}

	evicted, err := capmon.AgeBasedEvict(dir, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("AgeBasedEvict: %v", err)
	}
	if evicted == 0 {
		t.Error("expected at least one eviction")
	}
	if capmon.IsCached(dir, "windsurf", "llms-full") {
		t.Error("evicted entry should not be cached")
	}
}

func TestWriteAndReadRunManifest(t *testing.T) {
	dir := t.TempDir()
	m := capmon.RunManifest{
		RunID:     "test-run-001",
		StartedAt: time.Now().UTC(),
		ExitClass: capmon.ExitClean,
		Providers: map[string]capmon.ProviderStatus{
			"claude-code": {FetchStatus: "ok", ExtractStatus: "ok"},
		},
	}
	if err := capmon.WriteRunManifest(dir, m); err != nil {
		t.Fatalf("WriteRunManifest: %v", err)
	}

	got, err := capmon.ReadLastRunManifest(dir)
	if err != nil {
		t.Fatalf("ReadLastRunManifest: %v", err)
	}
	if got.RunID != m.RunID {
		t.Errorf("RunID: got %q, want %q", got.RunID, m.RunID)
	}
	if got.ExitClass != capmon.ExitClean {
		t.Errorf("ExitClass: got %d, want %d", got.ExitClass, capmon.ExitClean)
	}
}

func TestReadLastRunManifest_Missing(t *testing.T) {
	dir := t.TempDir()
	_, err := capmon.ReadLastRunManifest(dir)
	if err == nil {
		t.Error("expected error for missing manifest")
	}
}

func sha256Hash(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}
