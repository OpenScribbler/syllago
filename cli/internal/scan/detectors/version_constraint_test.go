package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestVersionConstraintGoGenerics(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Go 1.17 project — predates generics (introduced in 1.18).
	gomod := "module example.com/old\n\ngo 1.17\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	// Source file that uses generics syntax.
	src := `package main

func Map[T any](s []T, f func(T) T) []T {
	r := make([]T, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := VersionConstraint{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for generics constraint violation")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestVersionConstraintGoClean(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	// Go 1.22 project — generics are fine.
	gomod := "module example.com/modern\n\ngo 1.22\n"
	os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(gomod), 0644)

	src := `package main

func Map[T any](s []T, f func(T) T) []T {
	r := make([]T, len(s))
	for i, v := range s {
		r[i] = f(v)
	}
	return r
}
`
	os.WriteFile(filepath.Join(tmp, "main.go"), []byte(src), 0644)

	det := VersionConstraint{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for Go 1.22 with generics, got %d", len(sections))
	}
}
