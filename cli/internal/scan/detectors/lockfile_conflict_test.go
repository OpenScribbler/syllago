package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestLockFileConflictDetected(t *testing.T) {
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "package-lock.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(tmp, "yarn.lock"), []byte(""), 0644)

	det := LockFileConflict{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a surprise section for conflicting lock files")
	}
	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Source != "lock-file-conflict" {
		t.Errorf("source = %q, want lock-file-conflict", ts.Source)
	}
}

func TestLockFileConflictNone(t *testing.T) {
	tmp := t.TempDir()
	// Only one lock file — no conflict
	os.WriteFile(filepath.Join(tmp, "package-lock.json"), []byte("{}"), 0644)

	det := LockFileConflict{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for single lock file, got %d", len(sections))
	}
}
