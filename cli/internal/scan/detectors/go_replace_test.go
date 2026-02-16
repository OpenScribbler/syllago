package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestGoReplaceDetectsLocalPath(t *testing.T) {
	tmp := t.TempDir()

	gomod := `module example.com/myproject

go 1.22

require example.com/lib v1.0.0

replace example.com/lib => ../lib
`
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	det := GoReplace{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for local path replacement")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestGoReplaceCleanGoMod(t *testing.T) {
	tmp := t.TempDir()

	gomod := `module example.com/myproject

go 1.22

require example.com/lib v1.0.0
`
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	det := GoReplace{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean go.mod, got %d", len(sections))
	}
}
