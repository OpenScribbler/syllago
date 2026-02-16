package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestClaudeEmitterBasic(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TechStackSection{
				Origin:          model.OriginAuto,
				Title:           "Tech Stack",
				Language:        "Go",
				LanguageVersion: "1.22",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(output, "<!-- nesco:auto:tech-stack -->") {
		t.Error("missing opening boundary marker")
	}
	if !strings.Contains(output, "<!-- /nesco:auto:tech-stack -->") {
		t.Error("missing closing boundary marker")
	}
	if !strings.Contains(output, "Go 1.22") {
		t.Error("missing tech stack content")
	}
}

func TestClaudeEmitterSkipsHumanSections(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatCurated,
				Origin:   model.OriginHuman,
				Title:    "Architecture Notes",
				Body:     "This should not appear in output",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if strings.Contains(output, "Architecture Notes") {
		t.Error("human section should not be emitted")
	}
}

func TestClaudeEmitterSurprise(t *testing.T) {
	emitter := ClaudeEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test-project",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Competing Test Frameworks",
				Body:     "Both Jest and Vitest are configured.",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(output, "<!-- nesco:auto:surprise -->") {
		t.Error("missing surprise boundary marker")
	}
	if !strings.Contains(output, "Competing Test Frameworks") {
		t.Error("missing surprise title")
	}
}
