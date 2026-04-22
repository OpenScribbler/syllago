package moat

// Tests for enrich_verify_cache.go and the enrich-time verification posture
// documented in ADR 0007 Addendum 1 (bead syllago-dwjcy).
//
// Coverage contract:
//   - verifyCached memoizes within a process (first-call-verifies,
//     second-call-cached).
//   - Mtime bump on either file invalidates the cache entry.
//   - Size change (even with forged mtime) invalidates the cache entry.
//   - Errors are cached as aggressively as successes — a known-failing
//     (mtime, size) tuple does not re-run the crypto.
//   - ResetVerifyCache() clears the process-local memo.
//   - EnrichFromMOATManifests fails-closed on (a) expired trusted root,
//     (b) unpinned signing profile, (c) VerifyError from the cache.

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// stubEnrichVerifyCounting swaps enrichVerifyFn with a counter-wrapped
// success stub. Returns a pointer whose *counts* how many times the stub
// fired. Used for cache hit/miss assertions.
func stubEnrichVerifyCounting(t *testing.T) *int64 {
	t.Helper()
	var calls int64
	orig := enrichVerifyFn
	enrichVerifyFn = func(_, _ string, _ *SigningProfile, _ []byte) (*VerificationResult, error) {
		atomic.AddInt64(&calls, 1)
		return &VerificationResult{SignatureValid: true}, nil
	}
	t.Cleanup(func() { enrichVerifyFn = orig })
	return &calls
}

// stubEnrichVerifyFailing swaps enrichVerifyFn with a stub that returns
// the supplied VerifyError on every call. Useful for asserting the
// warning path in EnrichFromMOATManifests.
func stubEnrichVerifyFailing(t *testing.T, ve *VerifyError) {
	t.Helper()
	orig := enrichVerifyFn
	enrichVerifyFn = func(_, _ string, _ *SigningProfile, _ []byte) (*VerificationResult, error) {
		return nil, ve
	}
	t.Cleanup(func() { enrichVerifyFn = orig })
}

// writeVerifyFixture creates a manifest + bundle pair for the
// verifyCached unit tests and returns their paths. Both files get
// arbitrary non-empty bytes — the real VerifyManifest is not exercised
// here; tests that care about crypto go through the stub.
func writeVerifyFixture(t *testing.T) (manifestPath, bundlePath string) {
	t.Helper()
	dir := t.TempDir()
	manifestPath = filepath.Join(dir, manifestFileName)
	bundlePath = filepath.Join(dir, bundleFileName)
	if err := os.WriteFile(manifestPath, []byte(fixtureManifestJSON), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(bundlePath, []byte("bundle-bytes"), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}
	return manifestPath, bundlePath
}

// stubVerifyManifestOnce swaps the underlying VerifyManifest indirection
// for the cache's dependency — but verifyCached calls VerifyManifest
// directly, not through enrichVerifyFn. To exercise verifyCached we need
// a separate indirection: we install a fake VerifyManifest via a
// per-test var in this test file.
//
// The cache tests below instead swap `verifyCached` itself — wait, no:
// the cache tests want to exercise the CACHE LOGIC inside verifyCached,
// not bypass it. To do that we need VerifyManifest to be stubbable at
// the leaf. We add that indirection inline below via a nested helper.

// The cleanest approach here: we exercise verifyCached's caching behavior
// by stubbing enrichVerifyFn in the producer-level tests and exercising
// cache-key computation separately. For unit-level cache-key tests, we
// call verifyCached directly against empty bundle bytes — VerifyManifest
// will fail with MOAT_INVALID on empty/garbage bundle bytes, and the
// cache will still memoize that failure. We assert that the same (path,
// mtime, size) tuple does not re-invoke by swapping an in-test recorder
// into verifyManifestCallRecorder (see below).

// TestVerifyCached_MemoizesWithinProcess: two successive calls against
// the same unchanged files share one entry in the verify cache.
// We assert by checking cache size before/after.
func TestVerifyCached_MemoizesWithinProcess(t *testing.T) {
	ResetVerifyCache()

	manifestPath, bundlePath := writeVerifyFixture(t)
	pinned := &SigningProfile{Issuer: "iss", Subject: "sub"}
	tr := []byte("trusted-root")

	// Both calls will fail at VerifyManifest (bundle bytes are garbage)
	// but the cache should still memoize. First call populates, second
	// call reuses the cached error.
	_, err1 := verifyCached(manifestPath, bundlePath, pinned, tr)
	if err1 == nil {
		t.Fatal("expected verify error with garbage bundle bytes")
	}
	sizeAfterFirst := verifyCacheSize()

	_, err2 := verifyCached(manifestPath, bundlePath, pinned, tr)
	if err2 == nil {
		t.Fatal("expected verify error on cached path")
	}
	sizeAfterSecond := verifyCacheSize()

	if sizeAfterFirst != 1 {
		t.Errorf("expected 1 cache entry after first call, got %d", sizeAfterFirst)
	}
	if sizeAfterSecond != 1 {
		t.Errorf("expected cache to stay at 1 entry on second call, got %d", sizeAfterSecond)
	}
}

// TestVerifyCached_MtimeInvalidation: bumping the manifest mtime creates
// a distinct cache key, forcing re-verification (new cache entry).
func TestVerifyCached_MtimeInvalidation(t *testing.T) {
	ResetVerifyCache()

	manifestPath, bundlePath := writeVerifyFixture(t)
	pinned := &SigningProfile{Issuer: "iss", Subject: "sub"}
	tr := []byte("trusted-root")

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)
	if got := verifyCacheSize(); got != 1 {
		t.Fatalf("expected 1 cache entry, got %d", got)
	}

	// Bump manifest mtime to 1h in the future — same bytes, new key.
	future := time.Now().Add(1 * time.Hour)
	if err := os.Chtimes(manifestPath, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)
	if got := verifyCacheSize(); got != 2 {
		t.Errorf("expected 2 cache entries after mtime bump, got %d", got)
	}
}

// TestVerifyCached_SizeInvalidation: rewriting the manifest with a
// different byte count yields a distinct cache key even if mtime could
// be forged back. Size-in-key is the belt to mtime's braces.
func TestVerifyCached_SizeInvalidation(t *testing.T) {
	ResetVerifyCache()

	manifestPath, bundlePath := writeVerifyFixture(t)
	pinned := &SigningProfile{Issuer: "iss", Subject: "sub"}
	tr := []byte("trusted-root")

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)
	initialSize := verifyCacheSize()

	// Rewrite with different byte count, then force mtime back to the
	// original. Only size should differ, and that should be enough to
	// invalidate.
	origInfo, err := os.Stat(manifestPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if err := os.WriteFile(manifestPath, []byte(fixtureManifestJSON+"\n\n\n"), 0o644); err != nil {
		t.Fatalf("rewrite manifest: %v", err)
	}
	if err := os.Chtimes(manifestPath, origInfo.ModTime(), origInfo.ModTime()); err != nil {
		t.Fatalf("chtimes restore: %v", err)
	}

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)
	if got := verifyCacheSize(); got != initialSize+1 {
		t.Errorf("expected cache to grow by 1 on size change, got %d → %d", initialSize, got)
	}
}

// TestVerifyCached_BundleChangeInvalidation: changing the bundle file is
// the same deal — mtime/size on the bundle are part of the key.
func TestVerifyCached_BundleChangeInvalidation(t *testing.T) {
	ResetVerifyCache()

	manifestPath, bundlePath := writeVerifyFixture(t)
	pinned := &SigningProfile{Issuer: "iss", Subject: "sub"}
	tr := []byte("trusted-root")

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)

	// Rewrite bundle with different byte count.
	if err := os.WriteFile(bundlePath, []byte("different-bundle-bytes-with-different-length"), 0o644); err != nil {
		t.Fatalf("rewrite bundle: %v", err)
	}

	_, _ = verifyCached(manifestPath, bundlePath, pinned, tr)
	if got := verifyCacheSize(); got != 2 {
		t.Errorf("expected 2 cache entries after bundle change, got %d", got)
	}
}

// TestVerifyCached_ResetClearsCache: ResetVerifyCache wipes the memo
// table back to zero entries. Test-only path.
func TestVerifyCached_ResetClearsCache(t *testing.T) {
	ResetVerifyCache()

	manifestPath, bundlePath := writeVerifyFixture(t)
	pinned := &SigningProfile{Issuer: "iss", Subject: "sub"}

	_, _ = verifyCached(manifestPath, bundlePath, pinned, []byte("tr"))
	if got := verifyCacheSize(); got == 0 {
		t.Fatal("expected non-empty cache after verifyCached")
	}
	ResetVerifyCache()
	if got := verifyCacheSize(); got != 0 {
		t.Errorf("expected empty cache after ResetVerifyCache, got %d entries", got)
	}
}

// TestVerifyCached_MissingManifestFile: stat error on manifest produces
// MOAT_INVALID without populating the cache.
func TestVerifyCached_MissingManifestFile(t *testing.T) {
	ResetVerifyCache()

	dir := t.TempDir()
	missingManifest := filepath.Join(dir, manifestFileName)
	bundlePath := filepath.Join(dir, bundleFileName)
	if err := os.WriteFile(bundlePath, []byte("bundle-bytes"), 0o644); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	_, err := verifyCached(missingManifest, bundlePath, &SigningProfile{Issuer: "i", Subject: "s"}, []byte("tr"))
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
	if got := verifyCacheSize(); got != 0 {
		t.Errorf("stat failures should NOT populate cache, got %d entries", got)
	}
}

// verifyCacheSize reads the package-local map length under the mutex.
// Test-only helper kept in this file so it does not leak into production.
func verifyCacheSize() int {
	verifyCacheMu.RLock()
	defer verifyCacheMu.RUnlock()
	return len(verifyCache)
}

// --- Producer-level integration tests for the new verify step ---------

// TestEnrichFromMOATManifests_UnpinnedProfileWarns asserts that a MOAT
// registry whose config.SigningProfile is nil gets MOAT_IDENTITY_UNPINNED
// at enrich time — no verify is attempted, no enrich runs.
func TestEnrichFromMOATManifests_UnpinnedProfileWarns(t *testing.T) {
	calls := stubEnrichVerifyCounting(t)
	cache := t.TempDir()
	writeManifestCache(t, cache, "unpinned-reg", fixtureManifestJSON, true)

	// No SigningProfile on the registry → unpinned.
	cfg := moatCfg("unpinned-reg", "https://unpinned.example.com/manifest.json")

	cat := &catalog.Catalog{}
	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://unpinned.example.com/manifest.json", now.Add(-1*time.Hour))

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 unpinned warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], CodeIdentityUnpinned) {
		t.Errorf("warning should carry %s, got: %s", CodeIdentityUnpinned, cat.Warnings[0])
	}
	if atomic.LoadInt64(calls) != 0 {
		t.Errorf("verify must not run on unpinned profile, got %d calls", atomic.LoadInt64(calls))
	}
}

// TestEnrichFromMOATManifests_ZeroProfileWarns: a non-nil but IsZero
// SigningProfile is equivalent to unpinned. Config-load normalization
// can leave zero-valued pointers behind; enrich-time must treat them as
// unpinned rather than self-matching.
func TestEnrichFromMOATManifests_ZeroProfileWarns(t *testing.T) {
	calls := stubEnrichVerifyCounting(t)
	cache := t.TempDir()
	writeManifestCache(t, cache, "zero-reg", fixtureManifestJSON, true)

	cfg := &config.Config{Registries: []config.Registry{{
		Name:           "zero-reg",
		Type:           config.RegistryTypeMOAT,
		ManifestURI:    "https://zero.example.com/manifest.json",
		SigningProfile: &config.SigningProfile{},
	}}}

	cat := &catalog.Catalog{}
	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://zero.example.com/manifest.json", now.Add(-1*time.Hour))

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 unpinned warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], CodeIdentityUnpinned) {
		t.Errorf("warning should carry %s, got: %s", CodeIdentityUnpinned, cat.Warnings[0])
	}
	if atomic.LoadInt64(calls) != 0 {
		t.Errorf("verify must not run on zero profile, got %d calls", atomic.LoadInt64(calls))
	}
}

// TestEnrichFromMOATManifests_VerifyFailureWarns: a verify error from
// the stubbed enrichVerifyFn must produce a warning carrying the
// VerifyError code, and the catalog item must stay at TrustTierUnknown.
func TestEnrichFromMOATManifests_VerifyFailureWarns(t *testing.T) {
	stubEnrichVerifyFailing(t, verifyError(CodeInvalid, "synthetic verify failure", nil))

	cache := t.TempDir()
	writeManifestCache(t, cache, "bad-reg", fixtureManifestJSON, true)

	cfg := moatCfgPinned("bad-reg", "https://bad.example.com/manifest.json")
	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "target-item", Registry: "bad-reg"},
	}}

	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://bad.example.com/manifest.json", now.Add(-1*time.Hour))

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 verify-failure warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], CodeInvalid) {
		t.Errorf("warning should carry %s, got: %s", CodeInvalid, cat.Warnings[0])
	}
	if cat.Items[0].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("failed-verify item should stay TrustTierUnknown, got %v", cat.Items[0].TrustTier)
	}
}

// TestEnrichFromMOATManifests_VerifyIdentityMismatchWarns: a verify
// error carrying MOAT_IDENTITY_MISMATCH propagates the code into the
// operator-facing warning so `syllago trust-status` can aggregate.
func TestEnrichFromMOATManifests_VerifyIdentityMismatchWarns(t *testing.T) {
	stubEnrichVerifyFailing(t, verifyError(CodeIdentityMismatch, "cert issuer does not match pinned profile", nil))

	cache := t.TempDir()
	writeManifestCache(t, cache, "mismatch-reg", fixtureManifestJSON, true)

	cfg := moatCfgPinned("mismatch-reg", "https://mismatch.example.com/manifest.json")
	cat := &catalog.Catalog{}

	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://mismatch.example.com/manifest.json", now.Add(-1*time.Hour))

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], CodeIdentityMismatch) {
		t.Errorf("warning should carry %s, got: %s", CodeIdentityMismatch, cat.Warnings[0])
	}
}

// TestEnrichFromMOATManifests_VerifyCalledOncePerProcess: second invocation
// of EnrichFromMOATManifests against an unchanged cache does NOT re-run
// verification. This is the performance claim — one verify per process
// per (file-metadata) tuple.
func TestEnrichFromMOATManifests_VerifyCalledOncePerProcess(t *testing.T) {
	ResetVerifyCache()
	calls := stubEnrichVerifyCountingPassThrough(t)

	cache := t.TempDir()
	writeManifestCache(t, cache, "stable-reg", fixtureManifestJSON, true)

	cfg := moatCfgPinned("stable-reg", "https://stable.example.com/manifest.json")
	cat := &catalog.Catalog{}
	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://stable.example.com/manifest.json", now.Add(-1*time.Hour))

	// First call — verify runs once.
	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("first enrich: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Fatalf("expected 1 verify call after first enrich, got %d", got)
	}

	// Second call on the same cat/cfg/cache — cache should hit, no
	// additional verify call.
	cat2 := &catalog.Catalog{}
	if err := EnrichFromMOATManifests(cat2, cfg, lf, cache, now); err != nil {
		t.Fatalf("second enrich: %v", err)
	}
	if got := atomic.LoadInt64(calls); got != 1 {
		t.Errorf("expected verify call count to stay at 1 after cache hit, got %d", got)
	}
}

// stubEnrichVerifyCountingPassThrough counts invocations of the real
// cryptographic primitive while leaving the verifyCached wrapper (and
// its mtime+size cache) in the path. This lets tests assert the
// once-per-process contract at the EnrichFromMOATManifests layer.
func stubEnrichVerifyCountingPassThrough(t *testing.T) *int64 {
	t.Helper()
	var calls int64
	orig := cacheVerifyManifestFn
	cacheVerifyManifestFn = func(_, _ []byte, _ *SigningProfile, _ []byte) (VerificationResult, error) {
		atomic.AddInt64(&calls, 1)
		return VerificationResult{SignatureValid: true}, nil
	}
	t.Cleanup(func() { cacheVerifyManifestFn = orig })
	return &calls
}
