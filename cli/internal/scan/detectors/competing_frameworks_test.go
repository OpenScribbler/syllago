package detectors

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestCompetingFrameworksDetects(t *testing.T) {
	det := CompetingFrameworks{}
	sections, err := det.Detect("testdata/surprises/competing-frameworks")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 surprise section for competing test frameworks")
	}

	// Verify it's a TextSection with CatSurprise.
	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Origin != model.OriginAuto {
		t.Errorf("origin = %q, want %q", ts.Origin, model.OriginAuto)
	}
}

func TestCompetingFrameworksCleanProject(t *testing.T) {
	det := CompetingFrameworks{}
	// go-project has no competing frameworks — just cobra and bubbletea
	// which are in different categories.
	sections, err := det.Detect("testdata/techstack/go-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 surprise sections for clean Go project, got %d", len(sections))
	}
}

func TestCompetingFrameworksEmptyDir(t *testing.T) {
	det := CompetingFrameworks{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
