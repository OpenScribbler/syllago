package emit

import (
	"strings"
	"testing"
	"time"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestCursorEmitterFrontmatter(t *testing.T) {
	t.Parallel()
	emitter := CursorEmitter{}
	doc := model.ContextDocument{
		ProjectName: "test",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TechStackSection{
				Origin: model.OriginAuto, Title: "Tech Stack",
				Language: "TypeScript", LanguageVersion: "5.3",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.HasPrefix(output, "---\n") {
		t.Error("Cursor output should start with YAML frontmatter")
	}
	if !strings.Contains(output, "alwaysApply: true") {
		t.Error("missing alwaysApply in frontmatter")
	}
	if !strings.Contains(output, "# nesco:auto:tech-stack") {
		t.Error("missing YAML-style boundary marker")
	}
}

func TestGenericEmitter(t *testing.T) {
	t.Parallel()
	emitter := GenericMarkdownEmitter{ProviderSlug: "gemini-cli", FileName: "GEMINI.md"}
	doc := model.ContextDocument{
		ProjectName: "test",
		ScanTime:    time.Now(),
		Sections: []model.Section{
			model.TextSection{
				Category: model.CatSurprise,
				Origin:   model.OriginAuto,
				Title:    "Mixed naming",
				Body:     "camelCase and kebab-case both used.",
			},
		},
	}

	output, err := emitter.Emit(doc)
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}

	if !strings.Contains(output, "<!-- nesco:auto:surprise -->") {
		t.Error("missing HTML boundary marker")
	}
}

func TestEmitterForProviderReturnsCorrectTypes(t *testing.T) {
	tests := []struct {
		slug       string
		wantName   string
		wantFormat string
	}{
		{"claude-code", "claude-code", "md"},
		{"cursor", "cursor", "mdc"},
		{"gemini-cli", "gemini-cli", "md"},
		{"codex", "codex", "md"},
		{"windsurf", "windsurf", "md"},
		{"unknown", "unknown", "md"},
	}
	for _, tt := range tests {
		e := EmitterForProvider(tt.slug)
		if e == nil {
			t.Errorf("EmitterForProvider(%q) returned nil", tt.slug)
			continue
		}
		if e.Name() != tt.wantName {
			t.Errorf("EmitterForProvider(%q).Name() = %q, want %q", tt.slug, e.Name(), tt.wantName)
		}
		if e.Format() != tt.wantFormat {
			t.Errorf("EmitterForProvider(%q).Format() = %q, want %q", tt.slug, e.Format(), tt.wantFormat)
		}
	}
}
