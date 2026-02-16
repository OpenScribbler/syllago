package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestLinterExtractionPrettier(t *testing.T) {
	tmp := t.TempDir()

	prettierrc := `{"semi": false, "singleQuote": true, "tabWidth": 2}`
	os.WriteFile(filepath.Join(tmp, ".prettierrc"), []byte(prettierrc), 0644)

	det := LinterExtraction{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for Prettier config")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}

	// This is a conventions detector, not a surprise detector.
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if !strings.Contains(ts.Body, "semicolons: no") {
		t.Errorf("body should mention 'semicolons: no', got: %s", ts.Body)
	}
	if !strings.Contains(ts.Body, "quotes: single") {
		t.Errorf("body should mention 'quotes: single', got: %s", ts.Body)
	}
	if !strings.Contains(ts.Body, "indent: 2 spaces") {
		t.Errorf("body should mention 'indent: 2 spaces', got: %s", ts.Body)
	}
}

func TestLinterExtractionNoConfig(t *testing.T) {
	det := LinterExtraction{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for dir without config files, got %d", len(sections))
	}
}
