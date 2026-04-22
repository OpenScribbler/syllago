package moat

// Process-local verification cache for enrich-time manifest re-verify.
// See ADR 0007 Addendum 1 (bead syllago-dwjcy) for the full rationale.
//
// The cache's purpose is to close the same-user local-write gap between
// `syllago registry sync` runs without paying the full sigstore cost on
// every TUI rescan. Concretely:
//
//   - `EnrichFromMOATManifests` is invoked on every R-key rescan, every
//     install completion, every tab change — potentially hundreds of times
//     per TUI session.
//   - Running VerifyManifest (Fulcio chain + Rekor inclusion proof +
//     numeric-ID match) on every call is wasted work when the cache files
//     have not changed.
//   - Skipping verification entirely leaves the same-user local-write gap:
//     an attacker with write access to `~/.cache/syllago/moat/` between
//     syncs can corrupt the cached manifest and the TUI will render trust
//     decisions against the corrupted bytes.
//
// Strategy: run VerifyManifest exactly once per (manifest file, bundle
// file) pair per `syllago` process, memoize the result in a package-local
// map, and invalidate the entry whenever either file's mtime or size
// changes. A fresh process always re-verifies — the cache is intentionally
// not persisted (see ADR 0007 Addendum 1 §Cache scope for why persisting
// would recreate the MAC-protected-state problem the spec rejects).
//
// Cache key choice: (manifestPath, manifestMtime, manifestSize,
// bundleMtime, bundleSize) rather than a content hash. Hashing on every
// call would erode the performance win this cache exists to deliver; the
// stat-based key catches the things we need to catch (legitimate file
// refresh from a sibling sync; accidental corruption) and lets
// VerifyManifest itself provide the cryptographic binding when the key
// changes.

import (
	"fmt"
	"os"
	"sync"
)

// verifyCacheKey identifies a single (manifest, bundle) pair by path plus
// file-metadata observed at read time. Any change to either file flips the
// key and forces re-verification.
type verifyCacheKey struct {
	manifestPath  string
	manifestMtime int64 // UnixNano — truncating to seconds would miss fast-succession writes in tests
	manifestSize  int64
	bundleMtime   int64
	bundleSize    int64
}

// verifyCacheEntry is stored by value (map value). Both a success and a
// failure are worth memoizing — repeatedly re-running a known-failing
// verification wastes the same CPU as re-running a known-passing one,
// and a caller looping over a corrupt cache should not pay N× crypto
// cost for N warnings.
type verifyCacheEntry struct {
	result *VerificationResult
	err    error
}

var (
	verifyCacheMu sync.RWMutex
	verifyCache   = map[verifyCacheKey]verifyCacheEntry{}
)

// ResetVerifyCache clears the process-local verification cache. Test-only
// seam — production code never invalidates explicitly; cache invalidation
// is driven by file-metadata changes via the cache key.
func ResetVerifyCache() {
	verifyCacheMu.Lock()
	defer verifyCacheMu.Unlock()
	verifyCache = map[verifyCacheKey]verifyCacheEntry{}
}

// enrichVerifyFn is the indirection point for tests. Production callers go
// through verifyCached → VerifyManifest; tests swap in a stub that avoids
// real bundle fixtures. Mirrors `syncVerifyFn` in sync.go:56 so both
// enrich and sync paths present the same stubbing contract.
//
// Signature intentionally takes file paths rather than bytes so the
// memoization key can be computed without reading either file on a cache
// hit. A stubbed impl in tests may ignore the paths entirely.
var enrichVerifyFn = verifyCached

// cacheVerifyManifestFn is a leaf indirection verifyCached uses on cache
// miss. Tests swap this in to count how often crypto actually runs while
// still exercising the (stat → lookup → read → call → memoize) pipeline
// of verifyCached itself. Production code uses VerifyManifest directly.
var cacheVerifyManifestFn = VerifyManifest

// verifyCached is the memoized entry point. Stats both files to build the
// cache key, returns the cached outcome on a hit, and otherwise reads the
// bytes, calls VerifyManifest, and caches the result (including errors)
// before returning.
//
// pinned MUST be non-nil; passing nil is a caller bug caught by
// VerifyManifest's own nil-check (returns MOAT_IDENTITY_UNPINNED). We do
// not pre-check here because the caller in producer.go already warns +
// skips on a nil pinned profile before we are called.
//
// trustedRoot is passed through to VerifyManifest on cache miss; the
// trusted root is NOT part of the cache key because a single `syllago`
// process always sees the same bundled trusted root (changes ship in a
// new release, which is a new process).
func verifyCached(
	manifestPath, bundlePath string,
	pinned *SigningProfile,
	trustedRoot []byte,
) (*VerificationResult, error) {
	// Stat both files before touching the cache. A stat error is treated
	// like a verification failure — the producer caller will render the
	// warning and skip the registry. Using verifyError here keeps the
	// error taxonomy consistent with VerifyManifest itself.
	manifestInfo, err := os.Stat(manifestPath)
	if err != nil {
		return nil, verifyError(CodeInvalid,
			fmt.Sprintf("stat manifest %q", manifestPath), err)
	}
	bundleInfo, err := os.Stat(bundlePath)
	if err != nil {
		return nil, verifyError(CodeInvalid,
			fmt.Sprintf("stat bundle %q", bundlePath), err)
	}

	key := verifyCacheKey{
		manifestPath:  manifestPath,
		manifestMtime: manifestInfo.ModTime().UnixNano(),
		manifestSize:  manifestInfo.Size(),
		bundleMtime:   bundleInfo.ModTime().UnixNano(),
		bundleSize:    bundleInfo.Size(),
	}

	verifyCacheMu.RLock()
	entry, ok := verifyCache[key]
	verifyCacheMu.RUnlock()
	if ok {
		return entry.result, entry.err
	}

	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, verifyError(CodeInvalid,
			fmt.Sprintf("read manifest %q", manifestPath), err)
	}
	bundleBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, verifyError(CodeInvalid,
			fmt.Sprintf("read bundle %q", bundlePath), err)
	}

	result, verifyErr := cacheVerifyManifestFn(manifestBytes, bundleBytes, pinned, trustedRoot)

	// Cache both success and failure. A verify failure on a specific
	// (mtime, size) tuple is as stable as a success — repeating the same
	// work yields the same answer. Mutating the files changes the key and
	// forces re-verification on the next call.
	newEntry := verifyCacheEntry{err: verifyErr}
	if verifyErr == nil {
		r := result
		newEntry.result = &r
	}

	verifyCacheMu.Lock()
	verifyCache[key] = newEntry
	verifyCacheMu.Unlock()

	return newEntry.result, verifyErr
}
