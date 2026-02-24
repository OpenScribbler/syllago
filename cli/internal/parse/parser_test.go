package parse

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/OpenScribbler/nesco/cli/internal/catalog"
	"github.com/OpenScribbler/nesco/cli/internal/model"
)

func TestClaudeParserRule(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	rulePath := filepath.Join(tmp, "test-rule.md")
	os.WriteFile(rulePath, []byte("# Always use tabs\nTabs are the way."), 0644)

	parser := ClaudeParser{}
	sections, err := parser.ParseFile(DiscoveredFile{
		Path:        rulePath,
		ContentType: catalog.Rules,
		Provider:    "claude-code",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	ts := sections[0].(model.TextSection)
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if ts.Body != "# Always use tabs\nTabs are the way." {
		t.Errorf("body mismatch: %q", ts.Body)
	}
}

func TestCursorParserMDC(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	mdcPath := filepath.Join(tmp, "testing.mdc")
	content := "---\ndescription: Testing conventions\nalwaysApply: true\n---\nAlways use Jest for testing."
	os.WriteFile(mdcPath, []byte(content), 0644)

	parser := CursorParser{}
	sections, err := parser.ParseFile(DiscoveredFile{
		Path:        mdcPath,
		ContentType: catalog.Rules,
		Provider:    "cursor",
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if len(sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections))
	}
	ts := sections[0].(model.TextSection)
	if ts.Title != "Cursor Rule: Testing conventions" {
		t.Errorf("title = %q, want 'Cursor Rule: Testing conventions'", ts.Title)
	}
	if ts.Body != "Always use Jest for testing." {
		t.Errorf("body = %q", ts.Body)
	}
}

func TestParserForProvider(t *testing.T) {
	t.Parallel()
	tests := []struct {
		slug string
	}{
		{"claude-code"},
		{"cursor"},
		{"windsurf"},
	}
	for _, tt := range tests {
		p := ParserForProvider(tt.slug)
		if p == nil {
			t.Errorf("ParserForProvider(%q) returned nil", tt.slug)
		}
	}
}
