package capmon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// TestAllProviderSlugsRegistered asserts that every provider slug found in
// docs/provider-sources/ has a recognizer registered in recognizerRegistry.
// This test will fail until all per-provider recognizer stubs are written in Phase 5.
func TestAllProviderSlugsRegistered(t *testing.T) {
	// Walk docs/provider-sources/ relative to the repo root.
	// The test binary runs from cli/internal/capmon/, so we use ../../../docs/provider-sources/.
	sourcesDir := filepath.Join("..", "..", "..", "docs", "provider-sources")
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("cannot read docs/provider-sources/: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".yaml")
		if slug == "_template" {
			continue
		}
		// RecognizeContentTypeDotPaths with an unknown provider returns an empty map.
		// If the provider IS registered, calling with empty fields returns an empty map too.
		// We need a way to distinguish "registered but empty" from "not registered".
		// IsRecognizerRegistered is the exported check function (implemented in Task 1.3).
		if !capmon.IsRecognizerRegistered(slug) {
			t.Errorf("provider %q has no registered recognizer; add recognize_%s.go with init() registration",
				slug, strings.ReplaceAll(slug, "-", "_"))
		}
	}
}

// TestAllRegisteredRecognizersReturnMap calls RecognizeContentTypeDotPaths for every
// registered recognizer with empty fields and asserts the result is non-nil.
func TestAllRegisteredRecognizersReturnMap(t *testing.T) {
	sourcesDir := filepath.Join("..", "..", "..", "docs", "provider-sources")
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("cannot read docs/provider-sources/: %v", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".yaml")
		if slug == "_template" {
			continue
		}
		if !capmon.IsRecognizerRegistered(slug) {
			// Skip unregistered — TestAllProviderSlugsRegistered handles the failure.
			continue
		}
		result := capmon.RecognizeContentTypeDotPaths(slug, map[string]capmon.FieldValue{})
		if result == nil {
			t.Errorf("provider %q: RecognizeContentTypeDotPaths returned nil, want non-nil map", slug)
		}
	}
}
