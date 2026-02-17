package detectors

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestDeprecatedPatternDetected(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Create a legacy/ directory (counts as 1)
	os.MkdirAll(filepath.Join(tmp, "legacy"), 0755)

	// Create a source file with 6 markers (total = 7, threshold is >5)
	src := filepath.Join(tmp, "main.go")
	content := ""
	for i := 0; i < 6; i++ {
		content += fmt.Sprintf("// @deprecated function_%d\n", i)
	}
	os.WriteFile(src, []byte(content), 0644)

	det := DeprecatedPattern{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a surprise section for deprecation markers")
	}
	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestDeprecatedPatternBelowThreshold(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Only 3 markers — below the >5 threshold
	content := "@deprecated\n// DEPRECATED\nTODO: migrate\n"
	os.WriteFile(filepath.Join(tmp, "app.ts"), []byte(content), 0644)

	det := DeprecatedPattern{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections below threshold, got %d", len(sections))
	}
}
