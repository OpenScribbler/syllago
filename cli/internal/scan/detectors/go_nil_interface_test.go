package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestGoNilInterfaceDetectsPattern(t *testing.T) {
	tmp := t.TempDir()

	gomod := "module example.com/myproject\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	// Source with the typed nil return pattern.
	src := `package main

import "fmt"

type MyError struct {
	Code int
}

func (e *MyError) Error() string {
	return fmt.Sprintf("error code %d", e.Code)
}

func doWork() error {
	var err *MyError
	// some logic that might set err, but doesn't in this path
	return err
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := GoNilInterface{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for typed nil return pattern")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestGoNilInterfaceClean(t *testing.T) {
	tmp := t.TempDir()

	gomod := "module example.com/myproject\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	// Clean code — returns nil directly, not a typed nil.
	src := `package main

import "fmt"

func doWork() error {
	if true {
		return fmt.Errorf("something broke")
	}
	return nil
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := GoNilInterface{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean code, got %d", len(sections))
	}
}
