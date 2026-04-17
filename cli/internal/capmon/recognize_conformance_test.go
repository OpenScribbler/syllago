package capmon_test

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

// keyPattern is the canonical capability key regex enforced by MUST 2 in the
// recognizer conformance contract (see cli/internal/capmon/recognize.go package doc).
var keyPattern = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)*$`)

// TestRecognitionConformance_KeyRegex enforces RFC-2119 MUST 2:
// every capability key emitted by every registered recognizer MUST match
// ^[a-z_]+(\.[a-z_]+)*$ (lowercase, underscores, dot-separated segments).
//
// Driven by the live registry — adding a new recognizer automatically
// extends test coverage without editing this file.
func TestRecognitionConformance_KeyRegex(t *testing.T) {
	// Use a representative GoStruct payload so doc-only providers (which use
	// the GoStruct preset internally pre-PR4) also produce non-empty output.
	fields := map[string]capmon.FieldValue{
		"Skill.Name":          {Value: "name"},
		"Skill.Description":   {Value: "description"},
		"Skill.License":       {Value: "license"},
		"Skill.Compatibility": {Value: "compatibility"},
	}

	for _, slug := range registeredProviderSlugs(t) {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			result := capmon.RecognizeContentTypeDotPaths(slug, fields)
			for key := range result {
				if !keyPattern.MatchString(key) {
					t.Errorf("provider %q: key %q violates ^[a-z_]+(\\.[a-z_]+)*$", slug, key)
				}
			}
		})
	}
}

// TestRecognitionConformance_Determinism enforces RFC-2119 MUST 4:
// deeply-equal RecognitionContext MUST produce deeply-equal RecognitionResult.
// Runs each recognizer twice with identical inputs and asserts identical outputs.
func TestRecognitionConformance_Determinism(t *testing.T) {
	fields := map[string]capmon.FieldValue{
		"Skill.Name":        {Value: "name"},
		"Skill.Description": {Value: "description"},
		"Skill.License":     {Value: "license"},
	}

	for _, slug := range registeredProviderSlugs(t) {
		t.Run(slug, func(t *testing.T) {
			t.Parallel()
			a := capmon.RecognizeContentTypeDotPaths(slug, fields)
			b := capmon.RecognizeContentTypeDotPaths(slug, fields)
			if !reflect.DeepEqual(a, b) {
				t.Errorf("provider %q: non-deterministic output\n  call A: %v\n  call B: %v", slug, a, b)
			}
		})
	}
}

// TestRecognitionConformance_KindTagged asserts every registered recognizer
// declared a non-Unknown kind, OR is one of the deliberately-unsupported
// providers (cursor, zed) that returns an empty result.
func TestRecognitionConformance_KindTagged(t *testing.T) {
	allowedUnknown := map[string]bool{"cursor": true, "zed": true}
	for _, slug := range registeredProviderSlugs(t) {
		kind := capmon.RecognizerKindFor(slug)
		if kind == capmon.RecognizerKindUnknown && !allowedUnknown[slug] {
			t.Errorf("provider %q registered with RecognizerKindUnknown; declare a real kind in init()", slug)
		}
	}
}

// registeredProviderSlugs returns every provider slug from docs/provider-sources/
// that has a registered recognizer. Matches the discovery pattern used by
// recognize_registry_test.go to keep coverage automatic.
func registeredProviderSlugs(t *testing.T) []string {
	t.Helper()
	sourcesDir := filepath.Join("..", "..", "..", "docs", "provider-sources")
	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		t.Fatalf("cannot read docs/provider-sources/: %v", err)
	}
	var slugs []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		slug := strings.TrimSuffix(e.Name(), ".yaml")
		if slug == "_template" {
			continue
		}
		if !capmon.IsRecognizerRegistered(slug) {
			continue
		}
		slugs = append(slugs, slug)
	}
	return slugs
}
