package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// mockPathLookup implements ProviderPathLookup for testing.
type mockPathLookup struct {
	dirs map[string]string // slug → baseDir
}

func (m *mockPathLookup) BaseDir(slug string) string {
	return m.dirs[slug]
}

func TestDetectProviders_NoConfig(t *testing.T) {
	t.Parallel()
	providers := DetectProviders()
	if len(providers) != len(AllProviders) {
		t.Errorf("expected %d providers, got %d", len(AllProviders), len(providers))
	}
	// All providers should be present (detected or not)
	slugs := make(map[string]bool)
	for _, p := range providers {
		slugs[p.Slug] = true
	}
	for _, p := range AllProviders {
		if !slugs[p.Slug] {
			t.Errorf("missing provider %q in result", p.Slug)
		}
	}
}

func TestDetectProvidersWithResolver_NilLookup(t *testing.T) {
	t.Parallel()
	withResolver := DetectProvidersWithResolver(nil)
	direct := DetectProviders()

	if len(withResolver) != len(direct) {
		t.Fatalf("expected %d providers, got %d", len(direct), len(withResolver))
	}
	for i := range withResolver {
		if withResolver[i].Slug != direct[i].Slug {
			t.Errorf("provider[%d]: slug mismatch: %q vs %q", i, withResolver[i].Slug, direct[i].Slug)
		}
		if withResolver[i].Detected != direct[i].Detected {
			t.Errorf("provider %q: Detected mismatch: %v vs %v", withResolver[i].Slug, withResolver[i].Detected, direct[i].Detected)
		}
	}
}

func TestDetectProvidersWithResolver_CustomBaseDir_Exists(t *testing.T) {
	t.Parallel()
	// Use a provider that is unlikely to be installed in the test environment.
	// We pick the first provider whose Detect function would return false for a nonexistent home.
	// Use a temp dir as the custom base — it exists on disk.
	customBase := t.TempDir()

	// Find any provider slug to override.
	if len(AllProviders) == 0 {
		t.Skip("no providers defined")
	}
	target := AllProviders[0]

	lookup := &mockPathLookup{dirs: map[string]string{
		target.Slug: customBase,
	}}

	providers := DetectProvidersWithResolver(lookup)

	found := false
	for _, p := range providers {
		if p.Slug == target.Slug {
			found = true
			if !p.Detected {
				t.Errorf("provider %q: expected Detected=true when custom baseDir exists on disk", target.Slug)
			}
		}
	}
	if !found {
		t.Errorf("provider %q not found in results", target.Slug)
	}
}

func TestDetectProvidersWithResolver_CustomBaseDir_Missing(t *testing.T) {
	t.Parallel()
	// Point to a path that does not exist.
	missingPath := filepath.Join(t.TempDir(), "does-not-exist")

	if len(AllProviders) == 0 {
		t.Skip("no providers defined")
	}
	target := AllProviders[0]

	// Make sure standard detection would also fail for this provider by using a
	// nonexistent home dir override. We can't inject homeDir into DetectProviders,
	// so we simply assert: if the custom path is missing AND the standard Detect
	// returns false (which we verify separately), Detected must be false.
	lookup := &mockPathLookup{dirs: map[string]string{
		target.Slug: missingPath,
	}}

	providers := DetectProvidersWithResolver(lookup)

	// We can't know the standard detection result without running it, so get it.
	standard := DetectProviders()
	standardDetected := false
	for _, p := range standard {
		if p.Slug == target.Slug {
			standardDetected = p.Detected
		}
	}

	for _, p := range providers {
		if p.Slug == target.Slug {
			if !standardDetected && p.Detected {
				t.Errorf("provider %q: expected Detected=false when custom baseDir missing and standard detection fails", target.Slug)
			}
		}
	}
}

func TestDetectProvidersWithResolver_StandardAndCustom(t *testing.T) {
	t.Parallel()
	// A provider detected via standard means must stay detected regardless of resolver.
	// We can check this by looking at what DetectProviders() returns and confirming
	// DetectProvidersWithResolver(nil) agrees.

	standard := DetectProviders()
	withNilResolver := DetectProvidersWithResolver(nil)

	if len(standard) != len(withNilResolver) {
		t.Fatalf("length mismatch: %d vs %d", len(standard), len(withNilResolver))
	}
	for i := range standard {
		if standard[i].Detected != withNilResolver[i].Detected {
			t.Errorf("provider %q: standard=%v withNilResolver=%v",
				standard[i].Slug, standard[i].Detected, withNilResolver[i].Detected)
		}
	}
}

func TestDetectProvidersWithResolver_CustomDirDoesNotDowngradeStandard(t *testing.T) {
	t.Parallel()
	// A provider that IS detected via standard means should remain Detected=true
	// even if the custom baseDir points to a missing path.
	standard := DetectProviders()

	// Build a lookup that maps every provider to a nonexistent path.
	nonExistent := filepath.Join(t.TempDir(), "missing")
	dirs := make(map[string]string)
	for _, p := range AllProviders {
		dirs[p.Slug] = nonExistent
	}
	lookup := &mockPathLookup{dirs: dirs}

	withResolver := DetectProvidersWithResolver(lookup)

	for i, sp := range standard {
		if sp.Detected && !withResolver[i].Detected {
			t.Errorf("provider %q: was detected via standard but lost detection with missing custom path", sp.Slug)
		}
	}
}

func TestDetectProvidersWithResolver_CustomDirExistsForAll(t *testing.T) {
	t.Parallel()
	// Every provider with a custom dir pointing to a real path should be Detected=true.
	dirs := make(map[string]string)
	tempDirs := make(map[string]string)
	for _, p := range AllProviders {
		d := t.TempDir()
		tempDirs[p.Slug] = d
		dirs[p.Slug] = d
	}
	lookup := &mockPathLookup{dirs: dirs}

	providers := DetectProvidersWithResolver(lookup)

	for _, p := range providers {
		if _, hasCustom := dirs[p.Slug]; hasCustom {
			if !p.Detected {
				t.Errorf("provider %q: expected Detected=true when custom baseDir exists", p.Slug)
			}
		}
	}
}

func TestDetectProvidersWithResolver_EmptyCustomDir(t *testing.T) {
	t.Parallel()
	// Empty string from lookup is same as no override — falls back to standard detection.
	lookup := &mockPathLookup{dirs: map[string]string{}} // no entries → BaseDir returns ""

	withResolver := DetectProvidersWithResolver(lookup)
	standard := DetectProviders()

	if len(withResolver) != len(standard) {
		t.Fatalf("length mismatch: %d vs %d", len(withResolver), len(standard))
	}
	for i := range standard {
		if standard[i].Detected != withResolver[i].Detected {
			t.Errorf("provider %q: expected same as standard when custom dir empty", standard[i].Slug)
		}
	}
}

func TestDetectProvidersWithResolver_ReturnsAllProviders(t *testing.T) {
	t.Parallel()
	// Result must always contain all providers, never a subset.
	providers := DetectProvidersWithResolver(nil)
	if len(providers) != len(AllProviders) {
		t.Errorf("expected %d providers, got %d", len(AllProviders), len(providers))
	}
}

func TestDetectProvidersWithResolver_CustomPathStatCheck(t *testing.T) {
	t.Parallel()
	// Verify that the path existence check uses os.Stat correctly:
	// a path that exists as a regular file (not a dir) should still count as "exists".
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "provider-marker")
	if err := os.WriteFile(filePath, []byte("present"), 0644); err != nil {
		t.Fatal(err)
	}

	if len(AllProviders) == 0 {
		t.Skip("no providers defined")
	}
	target := AllProviders[0]
	lookup := &mockPathLookup{dirs: map[string]string{target.Slug: filePath}}

	providers := DetectProvidersWithResolver(lookup)
	for _, p := range providers {
		if p.Slug == target.Slug {
			if !p.Detected {
				t.Errorf("provider %q: expected Detected=true when custom path points to existing file", target.Slug)
			}
		}
	}
}
