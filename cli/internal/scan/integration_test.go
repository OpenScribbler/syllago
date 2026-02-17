package scan_test

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/scan"
	"github.com/holdenhewett/romanesco/cli/internal/scan/detectors"
)

func TestFullScanNoProject(t *testing.T) {
	t.Parallel()
	// Empty directory — all detectors should return gracefully
	scanner := scan.NewScanner(detectors.AllDetectors()...)
	result := scanner.Run(t.TempDir())

	if len(result.Document.Sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(result.Document.Sections))
	}
	// No warnings expected (detectors should return nil, not error)
	if len(result.Warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(result.Warnings), result.Warnings)
	}
}
