package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestDirectoryStructure(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)
	os.MkdirAll(filepath.Join(tmp, "tests"), 0755)
	os.MkdirAll(filepath.Join(tmp, "docs"), 0755)

	det := DirectoryStructure{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected directory structure section")
	}
	ds, ok := sections[0].(model.DirectoryStructureSection)
	if !ok {
		t.Fatalf("expected DirectoryStructureSection, got %T", sections[0])
	}
	if len(ds.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(ds.Entries))
	}

	conventions := make(map[string]string)
	for _, e := range ds.Entries {
		conventions[e.Path] = e.Convention
	}
	if conventions["src/"] != "source" {
		t.Errorf("src/ should be 'source', got %q", conventions["src/"])
	}
	if conventions["tests/"] != "test" {
		t.Errorf("tests/ should be 'test', got %q", conventions["tests/"])
	}
}

func TestDirectoryStructureSkipsHidden(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".git"), 0755)
	os.MkdirAll(filepath.Join(tmp, "src"), 0755)

	det := DirectoryStructure{}
	sections, _ := det.Detect(tmp)
	if len(sections) == 0 {
		t.Fatal("expected section")
	}
	ds := sections[0].(model.DirectoryStructureSection)
	for _, e := range ds.Entries {
		if e.Path == ".git/" {
			t.Error("hidden directories should be skipped")
		}
	}
}

func TestDirectoryStructureEmpty(t *testing.T) {
	t.Parallel()
	det := DirectoryStructure{}
	sections, err := det.Detect(t.TempDir())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for empty dir, got %d", len(sections))
	}
}
