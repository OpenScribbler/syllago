package errordocs

import (
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/output"
)

func TestCodeToSlug(t *testing.T) {
	tests := []struct {
		code, want string
	}{
		{"CATALOG_001", "catalog-001"},
		{"REGISTRY_008", "registry-008"},
		{"SYSTEM_002", "system-002"},
	}
	for _, tt := range tests {
		if got := codeToSlug(tt.code); got != tt.want {
			t.Errorf("codeToSlug(%q) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestSlugToCode(t *testing.T) {
	tests := []struct {
		slug, want string
	}{
		{"catalog-001", "CATALOG_001"},
		{"registry-008", "REGISTRY_008"},
		{"system-002", "SYSTEM_002"},
	}
	for _, tt := range tests {
		if got := slugToCode(tt.slug); got != tt.want {
			t.Errorf("slugToCode(%q) = %q, want %q", tt.slug, got, tt.want)
		}
	}
}

func TestCodeSlugRoundtrip(t *testing.T) {
	for _, code := range output.AllErrorCodes() {
		slug := codeToSlug(code)
		back := slugToCode(slug)
		if back != code {
			t.Errorf("roundtrip failed: %q -> %q -> %q", code, slug, back)
		}
	}
}

func TestExplain_MissingDoc(t *testing.T) {
	// Before doc files are written, Explain should return an error
	_, err := Explain("NONEXISTENT_999")
	if err == nil {
		t.Error("expected error for nonexistent code")
	}
}

// TestErrorCodeDocsParity enforces 1:1 correspondence between error code
// constants in output.AllErrorCodes() and doc files in the embedded FS.
// This test will fail until all 46 doc files are written (Phase 2).
func TestErrorCodeDocsParity(t *testing.T) {
	codes := output.AllErrorCodes()
	docCodes := ListCodes()

	// Build maps for comparison
	codeSet := make(map[string]bool, len(codes))
	for _, c := range codes {
		codeSet[c] = true
	}
	docSet := make(map[string]bool, len(docCodes))
	for _, c := range docCodes {
		docSet[c] = true
	}

	// Check for codes without docs
	var missingDocs []string
	for _, c := range codes {
		if !docSet[c] {
			missingDocs = append(missingDocs, c)
		}
	}

	// Check for orphan docs (doc files without a code constant)
	var orphanDocs []string
	for _, c := range docCodes {
		if !codeSet[c] {
			orphanDocs = append(orphanDocs, c)
		}
	}

	if len(missingDocs) > 0 {
		t.Errorf("%d error codes without doc files: %v", len(missingDocs), missingDocs)
	}
	if len(orphanDocs) > 0 {
		t.Errorf("%d orphan doc files without error codes: %v", len(orphanDocs), orphanDocs)
	}
}
