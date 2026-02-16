package detectors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestTestConventionMixed(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0755)

	// Create a mix: 3 .test.ts and 3 .spec.ts → 50/50 split, both >20%
	for _, name := range []string{"a.test.ts", "b.test.ts", "c.test.ts"} {
		os.WriteFile(filepath.Join(src, name), []byte("test"), 0644)
	}
	for _, name := range []string{"x.spec.ts", "y.spec.ts", "z.spec.ts"} {
		os.WriteFile(filepath.Join(src, name), []byte("test"), 0644)
	}

	det := TestConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a surprise section for mixed test naming")
	}
	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
}

func TestTestConventionConsistent(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src")
	os.MkdirAll(src, 0755)

	// All .test.ts — consistent naming, no surprise
	for _, name := range []string{"a.test.ts", "b.test.ts", "c.test.ts", "d.test.ts"} {
		os.WriteFile(filepath.Join(src, name), []byte("test"), 0644)
	}

	det := TestConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections for consistent naming, got %d", len(sections))
	}
}
