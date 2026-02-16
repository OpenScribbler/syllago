package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestGoInternalDetectsViolation(t *testing.T) {
	tmp := t.TempDir()

	// Create a Go project with an internal/ directory.
	gomod := "module example.com/myproject\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	os.MkdirAll(filepath.Join(tmp, "internal", "secret"), 0755)
	src := "package secret\n\nfunc API() string { return \"hidden\" }\n"
	os.WriteFile(filepath.Join(tmp, "internal", "secret", "api.go"), []byte(src), 0644)

	det := GoInternal{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected at least 1 section for internal/ directory")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestGoInternalSkipsNonGo(t *testing.T) {
	tmp := t.TempDir()

	// No go.mod — not a Go project.
	det := GoInternal{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for non-Go project, got %d", len(sections))
	}
}
