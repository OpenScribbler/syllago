package capmon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// CacheMeta is the metadata stored alongside raw.bin for each fetched source.
type CacheMeta struct {
	FetchedAt   time.Time `json:"fetched_at"`
	ContentHash string    `json:"content_hash"`
	FetchStatus string    `json:"fetch_status"`
	FetchMethod string    `json:"fetch_method"`
	Cached      bool      `json:"cached,omitempty"`
}

// CacheEntry is the full in-memory representation of a cached source.
type CacheEntry struct {
	Provider string
	SourceID string
	Raw      []byte
	Meta     CacheMeta
}

// SHA256Hex computes "sha256:<hex>" for the given bytes.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(h[:])
}

func cacheEntryDir(cacheRoot, provider, sourceID string) string {
	return filepath.Join(cacheRoot, provider, sourceID)
}

// WriteCacheEntry writes raw.bin and meta.json for one source.
func WriteCacheEntry(cacheRoot string, entry CacheEntry) error {
	dir := cacheEntryDir(cacheRoot, entry.Provider, entry.SourceID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir cache entry %s: %w", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw.bin"), entry.Raw, 0644); err != nil {
		return fmt.Errorf("write raw.bin: %w", err)
	}
	metaData, err := json.Marshal(entry.Meta)
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), metaData, 0644); err != nil {
		return fmt.Errorf("write meta.json: %w", err)
	}
	return nil
}

// ReadCacheEntry reads raw.bin and meta.json for one source.
func ReadCacheEntry(cacheRoot, provider, sourceID string) (*CacheEntry, error) {
	dir := cacheEntryDir(cacheRoot, provider, sourceID)
	raw, err := os.ReadFile(filepath.Join(dir, "raw.bin"))
	if err != nil {
		return nil, fmt.Errorf("read raw.bin: %w", err)
	}
	metaData, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil, fmt.Errorf("read meta.json: %w", err)
	}
	var meta CacheMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, fmt.Errorf("parse meta.json: %w", err)
	}
	return &CacheEntry{
		Provider: provider,
		SourceID: sourceID,
		Raw:      raw,
		Meta:     meta,
	}, nil
}

// IsCached returns true if meta.json exists for the given provider+sourceID.
func IsCached(cacheRoot, provider, sourceID string) bool {
	dir := cacheEntryDir(cacheRoot, provider, sourceID)
	_, err := os.Stat(filepath.Join(dir, "meta.json"))
	return err == nil
}

// AgeBasedEvict removes cache entries whose FetchedAt is older than maxAge.
// Returns the number of entries evicted.
func AgeBasedEvict(cacheRoot string, maxAge time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-maxAge)
	evicted := 0
	err := fs.WalkDir(os.DirFS(cacheRoot), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "meta.json" {
			return err
		}
		abs := filepath.Join(cacheRoot, path)
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil // skip unreadable entries
		}
		var meta CacheMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil // skip corrupt entries
		}
		if meta.FetchedAt.Before(cutoff) {
			entryDir := filepath.Dir(abs)
			if err := os.RemoveAll(entryDir); err != nil {
				return fmt.Errorf("evict %s: %w", entryDir, err)
			}
			evicted++
		}
		return nil
	})
	return evicted, err
}
