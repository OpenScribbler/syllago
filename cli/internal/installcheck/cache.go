package installcheck

import "sync"

// mtimeCacheEntry is the per-target cache shape (D16): process-local,
// invalidated on stat change or explicit InvalidateCache.
type mtimeCacheEntry struct {
	mtime int64
	size  int64
	state map[string]PerTargetState // libID -> state
}

// cache is process-local per D16 "no on-disk cache file".
var (
	cacheMu sync.Mutex
	cache   = map[string]mtimeCacheEntry{}
)

// cacheGet returns the cached per-library state for target when the stat
// (mtime+size) matches. Returns (nil, false) on miss or stale.
func cacheGet(target string, mtime, size int64) (map[string]PerTargetState, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	entry, ok := cache[target]
	if !ok {
		return nil, false
	}
	if entry.mtime != mtime || entry.size != size {
		return nil, false
	}
	return entry.state, true
}

// cachePut stores or overwrites the cache entry for target.
func cachePut(target string, mtime, size int64, state map[string]PerTargetState) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cache[target] = mtimeCacheEntry{mtime: mtime, size: size, state: state}
}

// InvalidateCache is called from install/uninstall paths that touch target.
func InvalidateCache(target string) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	delete(cache, target)
}
