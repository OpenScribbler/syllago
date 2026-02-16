package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestPythonLayoutDetectsSrcLayout(t *testing.T) {
	tmp := t.TempDir()

	// Python project with src layout.
	os.WriteFile(filepath.Join(tmp, "pyproject.toml"), []byte("[project]\nname = \"myapp\"\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "src", "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "src", "myapp", "__init__.py"), []byte(""), 0644)

	det := PythonLayout{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for src layout detection")
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
	if !strings.Contains(ts.Body, "src layout") {
		t.Errorf("body should mention src layout, got %q", ts.Body)
	}
}

func TestPythonLayoutDetectsFlatLayout(t *testing.T) {
	tmp := t.TempDir()

	// Python project with flat layout.
	os.WriteFile(filepath.Join(tmp, "setup.py"), []byte("from setuptools import setup\nsetup()\n"), 0644)
	os.MkdirAll(filepath.Join(tmp, "myapp"), 0755)
	os.WriteFile(filepath.Join(tmp, "myapp", "__init__.py"), []byte(""), 0644)

	det := PythonLayout{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for flat layout detection")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if !strings.Contains(ts.Body, "flat layout") {
		t.Errorf("body should mention flat layout, got %q", ts.Body)
	}
}

func TestPythonLayoutSkipsNonPython(t *testing.T) {
	tmp := t.TempDir()
	// Empty dir — no Python markers.

	det := PythonLayout{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for non-Python project, got %v", sections)
	}
}
