package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestGoCGODetectsImportC(t *testing.T) {
	tmp := t.TempDir()

	gomod := "module example.com/myproject\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	src := `package main

// #include <stdio.h>
import "C"

func main() {
	C.puts(C.CString("hello from C"))
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := GoCGO{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected sections for CGO usage")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestGoCGOCleanProject(t *testing.T) {
	tmp := t.TempDir()

	gomod := "module example.com/myproject\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	src := `package main

import "fmt"

func main() {
	fmt.Println("pure Go, no CGO")
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := GoCGO{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for clean project, got %d", len(sections))
	}
}
