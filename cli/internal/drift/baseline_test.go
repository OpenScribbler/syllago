package drift

import (
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestSaveAndLoadBaseline(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	doc := model.ContextDocument{
		ProjectName: "test",
		Sections: []model.Section{
			model.TechStackSection{
				Origin: model.OriginAuto, Title: "Tech Stack",
				Language: "Go", LanguageVersion: "1.22",
			},
		},
	}

	if err := SaveBaseline(tmp, doc); err != nil {
		t.Fatalf("SaveBaseline: %v", err)
	}

	loaded, err := LoadBaseline(tmp)
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}

	if len(loaded.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(loaded.Sections))
	}
	if loaded.Sections[0].Category != "tech-stack" {
		t.Errorf("category = %q, want tech-stack", loaded.Sections[0].Category)
	}
}

func TestBaselineNotExists(t *testing.T) {
	t.Parallel()
	if BaselineExists(t.TempDir()) {
		t.Error("baseline should not exist in empty dir")
	}
}
