package moat

// Tests for producer.go (ADR 0007 Phase 2c, bead syllago-lqas0).
//
// Coverage contract: hit every branch of EnrichFromMOATManifests (nil
// guards, invalid name rejection, missing manifest, missing bundle,
// parse failure, staleness trichotomy, multi-registry happy path) plus
// the two-line ScanAndEnrich composition so the factory does not
// silently swallow an enrich-phase warning.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/catalog"
	"github.com/OpenScribbler/syllago/cli/internal/config"
)

// writeManifestCache lays out a fixture cache entry at
// <root>/moat/registries/<name>/{manifest.json,signature.bundle}.
// The bundle body does not matter — producer.go only checks its
// presence. The manifest body MUST parse, so callers pass a known-good
// JSON string (typically minimalManifestJSON or fixtureManifestJSON).
func writeManifestCache(t *testing.T, root, name, manifestJSON string, withBundle bool) {
	t.Helper()
	dir := filepath.Join(root, manifestCacheDirName, manifestCacheSubDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir cache %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestFileName), []byte(manifestJSON), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if withBundle {
		if err := os.WriteFile(filepath.Join(dir, bundleFileName), []byte("bundle-bytes"), 0o644); err != nil {
			t.Fatalf("write bundle: %v", err)
		}
	}
}

// moatCfg returns a config.Config with a single MOAT registry named `name`
// pointed at manifestURI. Non-MOAT defaults fill everything else so tests
// only vary the dimensions they care about.
func moatCfg(name, manifestURI string) *config.Config {
	return &config.Config{
		Registries: []config.Registry{
			{
				Name:        name,
				Type:        config.RegistryTypeMOAT,
				ManifestURI: manifestURI,
			},
		},
	}
}

// --- Nil guards --------------------------------------------------------

func TestEnrichFromMOATManifests_NilCatalog(t *testing.T) {
	t.Parallel()
	err := EnrichFromMOATManifests(nil, &config.Config{}, &Lockfile{}, t.TempDir(), time.Now())
	if err == nil {
		t.Fatal("expected error for nil catalog")
	}
}

func TestEnrichFromMOATManifests_NilConfig(t *testing.T) {
	t.Parallel()
	err := EnrichFromMOATManifests(&catalog.Catalog{}, nil, &Lockfile{}, t.TempDir(), time.Now())
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

// TestEnrichFromMOATManifests_NoMOATRegistries confirms the fast-path:
// a config with only git registries is a no-op. No warnings, no I/O.
func TestEnrichFromMOATManifests_NoMOATRegistries(t *testing.T) {
	t.Parallel()
	cat := &catalog.Catalog{Items: []catalog.ContentItem{{Name: "a", Registry: "gitreg"}}}
	cfg := &config.Config{Registries: []config.Registry{
		{Name: "gitreg", Type: config.RegistryTypeGit, URL: "https://example.com/repo.git"},
	}}

	if err := EnrichFromMOATManifests(cat, cfg, &Lockfile{}, t.TempDir(), time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", cat.Warnings)
	}
	if cat.Items[0].TrustTier != catalog.TrustTierUnknown {
		t.Errorf("git-registry item tier mutated: %v", cat.Items[0].TrustTier)
	}
}

// --- Happy path: fresh cache → enrichment runs -------------------------

func TestEnrichFromMOATManifests_FreshCacheEnriches(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	writeManifestCache(t, cache, "example-reg", fixtureManifestJSON, true)

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")

	// Lockfile registers a fetched_at inside the 72h window so
	// CheckRegistry returns Fresh.
	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://registry.example.com/manifest.json", now.Add(-1*time.Hour))

	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "irrelevant", Registry: "example-reg"},
	}}

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// fixtureManifestJSON has no content entries, so the item stays Unknown
	// — but crucially, no warnings fired: the cache was read, parsed, and
	// CheckRegistry returned Fresh.
	if len(cat.Warnings) != 0 {
		t.Errorf("expected no warnings on fresh-cache happy path, got %v", cat.Warnings)
	}
}

// TestEnrichFromMOATManifests_FreshCacheWithContent proves a full
// end-to-end populate: manifest lists an item, catalog has the matching
// row, and the post-enrich row carries the expected tier.
func TestEnrichFromMOATManifests_FreshCacheWithContent(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	writeManifestCache(t, cache, "example-reg", manifestWithRevocations(t), true)

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")

	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://registry.example.com/manifest.json", now.Add(-1*time.Hour))

	cat := &catalog.Catalog{Items: []catalog.ContentItem{
		{Name: "public-tool", Registry: "example-reg"},
		{Name: "private-tool", Registry: "example-reg"},
	}}

	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat.Items[1].PrivateRepo != true {
		t.Errorf("private-tool PrivateRepo = %v, want true", cat.Items[1].PrivateRepo)
	}
	if cat.Items[0].PrivateRepo != false {
		t.Errorf("public-tool PrivateRepo = %v, want false", cat.Items[0].PrivateRepo)
	}
}

// --- Failure paths: each emits exactly one warning and skips enrich ----

func TestEnrichFromMOATManifests_MissingCache(t *testing.T) {
	t.Parallel()
	cache := t.TempDir() // empty cache
	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")
	cat := &catalog.Catalog{}

	if err := EnrichFromMOATManifests(cat, cfg, &Lockfile{}, cache, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if got := cat.Warnings[0]; !contains(got, "missing") {
		t.Errorf("warning should mention 'missing', got: %s", got)
	}
}

func TestEnrichFromMOATManifests_MissingBundle(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	// Write manifest WITHOUT the bundle file.
	writeManifestCache(t, cache, "example-reg", fixtureManifestJSON, false)

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")
	cat := &catalog.Catalog{}

	if err := EnrichFromMOATManifests(cat, cfg, &Lockfile{}, cache, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], "incomplete") {
		t.Errorf("warning should mention 'incomplete', got: %s", cat.Warnings[0])
	}
}

func TestEnrichFromMOATManifests_UnparseableManifest(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	writeManifestCache(t, cache, "example-reg", `{"schema_version":`, true) // truncated JSON

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")
	cat := &catalog.Catalog{}

	if err := EnrichFromMOATManifests(cat, cfg, &Lockfile{}, cache, time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], "unparseable") {
		t.Errorf("warning should mention 'unparseable', got: %s", cat.Warnings[0])
	}
}

// TestEnrichFromMOATManifests_Stale: lockfile fetched_at older than 72h.
// CheckRegistry should return StalenessStale → warning emitted, no enrich.
func TestEnrichFromMOATManifests_Stale(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	writeManifestCache(t, cache, "example-reg", fixtureManifestJSON, true)

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")
	now := time.Now().UTC()
	lf := &Lockfile{}
	// 73h old fetch — past the 72h DefaultStalenessThreshold.
	lf.SetRegistryFetchedAt("https://registry.example.com/manifest.json", now.Add(-73*time.Hour))

	cat := &catalog.Catalog{}
	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", cat.Warnings)
	}
	if !contains(cat.Warnings[0], "stale") {
		t.Errorf("warning should mention 'stale', got: %s", cat.Warnings[0])
	}
}

// TestEnrichFromMOATManifests_InvalidRegistryName: a MOAT registry whose
// name fails IsValidRegistryName skips enrichment with a warning. This is
// belt-and-braces — config-load should already have rejected it — but
// the producer does not trust upstream validation to have run.
func TestEnrichFromMOATManifests_InvalidRegistryName(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Registries: []config.Registry{{
		Name:        "../escape",
		Type:        config.RegistryTypeMOAT,
		ManifestURI: "https://registry.example.com/manifest.json",
	}}}

	cat := &catalog.Catalog{}
	if err := EnrichFromMOATManifests(cat, cfg, &Lockfile{}, t.TempDir(), time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %v", cat.Warnings)
	}
	if !contains(cat.Warnings[0], "invalid name") {
		t.Errorf("warning should mention 'invalid name', got: %s", cat.Warnings[0])
	}
}

// TestEnrichFromMOATManifests_MultiRegistryMixed: one registry Fresh,
// one Missing. First enriches, second warns. Proves per-registry
// independence — one failure does not mask another's success.
func TestEnrichFromMOATManifests_MultiRegistryMixed(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	writeManifestCache(t, cache, "good-reg", fixtureManifestJSON, true)
	// "bad-reg" intentionally has no cache.

	cfg := &config.Config{Registries: []config.Registry{
		{Name: "good-reg", Type: config.RegistryTypeMOAT, ManifestURI: "https://good.example.com/manifest.json"},
		{Name: "bad-reg", Type: config.RegistryTypeMOAT, ManifestURI: "https://bad.example.com/manifest.json"},
	}}
	now := time.Now().UTC()
	lf := &Lockfile{}
	lf.SetRegistryFetchedAt("https://good.example.com/manifest.json", now.Add(-1*time.Hour))

	cat := &catalog.Catalog{}
	if err := EnrichFromMOATManifests(cat, cfg, lf, cache, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cat.Warnings) != 1 {
		t.Fatalf("expected 1 warning (from bad-reg only), got %d: %v", len(cat.Warnings), cat.Warnings)
	}
	if !contains(cat.Warnings[0], "bad-reg") {
		t.Errorf("warning should name bad-reg, got: %s", cat.Warnings[0])
	}
}

// --- ScanAndEnrich composition -----------------------------------------

// TestScanAndEnrich_NilConfigStillScans proves the "cfg == nil" short-
// circuit inside ScanAndEnrich: callers that somehow pass nil cfg still
// get a scanned catalog (no MOAT enrichment, but no panic). This matches
// the robustness contract ScanWithGlobalAndRegistries itself provides.
func TestScanAndEnrich_NilConfigStillScans(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cat, err := ScanAndEnrich(nil, root, root, nil, &Lockfile{}, t.TempDir(), time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat == nil {
		t.Fatal("catalog is nil despite scan succeeding")
	}
}

// TestScanAndEnrich_PassesWarningsThrough: a scan that succeeds + a
// MOAT registry whose cache is missing → catalog returns with the
// warning appended. Proves enrich-phase warnings reach the caller
// through ScanAndEnrich (not swallowed).
func TestScanAndEnrich_PassesWarningsThrough(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cache := t.TempDir()

	cfg := moatCfg("example-reg", "https://registry.example.com/manifest.json")

	cat, err := ScanAndEnrich(cfg, root, root, nil, &Lockfile{}, cache, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat == nil {
		t.Fatal("catalog is nil")
	}
	if len(cat.Warnings) == 0 {
		t.Errorf("expected cache-missing warning to surface on cat.Warnings")
	}
}

// --- helper ------------------------------------------------------------

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		indexOf(haystack, needle) >= 0
}

// indexOf returns the first byte-index of needle in haystack, or -1.
// Tiny local impl avoids pulling strings into a file that otherwise
// doesn't need it.
func indexOf(haystack, needle string) int {
	if len(needle) == 0 {
		return 0
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
