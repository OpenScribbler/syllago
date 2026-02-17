package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestTSStrictnessFullStrict(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "strict": true
  }
}`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	det := TSStrictness{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for strict mode")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if !strings.Contains(ts.Body, "fully enabled") {
		t.Errorf("body should mention fully enabled, got: %s", ts.Body)
	}
}

func TestTSStrictnessPartial(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "noImplicitAny": true,
    "strictNullChecks": true
  }
}`
	os.WriteFile(filepath.Join(tmp, "tsconfig.json"), []byte(tsconfig), 0644)

	det := TSStrictness{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for partial strictness")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatConventions {
		t.Errorf("category = %q, want %q", ts.Category, model.CatConventions)
	}
	if !strings.Contains(ts.Body, "individually set") {
		t.Errorf("body should mention individually set flags, got: %s", ts.Body)
	}
}

func TestTSStrictnessSkipsNonTS(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	det := TSStrictness{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if sections != nil {
		t.Errorf("expected nil sections for non-TS project, got %d", len(sections))
	}
}
