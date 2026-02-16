package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestVersionMismatchNextMixedRouting(t *testing.T) {
	tmp := t.TempDir()

	// Next.js 14 project with both pages/ and app/ directories.
	pkg := `{"dependencies": {"next": "^14.1.0", "react": "^18.2.0"}}`
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkg), 0644)
	os.MkdirAll(filepath.Join(tmp, "pages"), 0755)
	os.MkdirAll(filepath.Join(tmp, "app"), 0755)

	det := VersionMismatch{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for mixed routing")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Title != "Next.js Mixed Routing" {
		t.Errorf("title = %q, want %q", ts.Title, "Next.js Mixed Routing")
	}
}

func TestVersionMismatchNoMixedRouting(t *testing.T) {
	tmp := t.TempDir()

	// Next.js 14 project with only app/ directory — no conflict.
	pkg := `{"dependencies": {"next": "^14.1.0", "react": "^18.2.0"}}`
	os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkg), 0644)
	os.MkdirAll(filepath.Join(tmp, "app"), 0755)

	det := VersionMismatch{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections when only app/ exists, got %d", len(sections))
	}
}
