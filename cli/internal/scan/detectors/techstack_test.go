package detectors

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestTechStackNode(t *testing.T) {
	t.Parallel()
	det := TechStack{}
	sections, err := det.Detect("testdata/techstack/node-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 section for Node project")
	}
	ts, ok := sections[0].(model.TechStackSection)
	if !ok {
		t.Fatalf("expected TechStackSection, got %T", sections[0])
	}
	if ts.Language != "TypeScript" {
		t.Errorf("language = %q, want TypeScript", ts.Language)
	}
	if ts.Framework != "Next.js" {
		t.Errorf("framework = %q, want Next.js", ts.Framework)
	}
}

func TestTechStackGo(t *testing.T) {
	t.Parallel()
	det := TechStack{}
	sections, err := det.Detect("testdata/techstack/go-project")
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 section for Go project")
	}
	ts, ok := sections[0].(model.TechStackSection)
	if !ok {
		t.Fatalf("expected TechStackSection, got %T", sections[0])
	}
	if ts.Language != "Go" {
		t.Errorf("language = %q, want Go", ts.Language)
	}
	if ts.LanguageVersion != "1.22.5" {
		t.Errorf("version = %q, want 1.22.5", ts.LanguageVersion)
	}
}

func TestTechStackEmptyDir(t *testing.T) {
	t.Parallel()
	det := TechStack{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
