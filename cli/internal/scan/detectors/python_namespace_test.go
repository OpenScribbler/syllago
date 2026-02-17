package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestPythonNamespaceDetectsMissingInit(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Python project marker.
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)

	// Package with __init__.py.
	os.MkdirAll(filepath.Join(tmp, "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)

	// Sub-package missing __init__.py.
	os.MkdirAll(filepath.Join(tmp, "myapp", "utils"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "helpers.py"), []byte("def helper(): pass\n"), 0644)

	det := PythonNamespace{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for missing __init__.py in utils/")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if ts.Origin != model.OriginAuto {
		t.Errorf("origin = %q, want %q", ts.Origin, model.OriginAuto)
	}
	if ts.Title != "Missing __init__.py" {
		t.Errorf("title = %q, want %q", ts.Title, "Missing __init__.py")
	}
}

func TestPythonNamespaceClean(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Python project marker.
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)

	// Package with proper __init__.py at every level.
	os.MkdirAll(filepath.Join(tmp, "myapp", "utils"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "utils", "helpers.py"), []byte("def helper(): pass\n"), 0644)

	det := PythonNamespace{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean project, got %d", len(sections))
	}
}

func TestPythonNamespaceExcludesTests(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Python project marker.
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)

	// tests/ dir without __init__.py — should NOT be flagged.
	os.MkdirAll(filepath.Join(tmp, "tests"), 0755)
	os.WriteFile(filepath.Join(tmp, "tests", "test_app.py"), []byte("def test_something(): pass\n"), 0644)

	// Also add a proper package so the walk has something to process.
	os.MkdirAll(filepath.Join(tmp, "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmp, "myapp", "app.py"), []byte("pass\n"), 0644)

	det := PythonNamespace{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections when only tests/ is missing __init__.py, got %d", len(sections))
	}
}
