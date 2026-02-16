package detectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/holdenhewett/romanesco/cli/internal/model"
)

func TestEnvConventionUndocumented(t *testing.T) {
	tmp := t.TempDir()

	// .env.example documents two vars.
	envExample := "DATABASE_URL=postgres://localhost/dev\nAPI_KEY=your-key-here\n"
	os.WriteFile(filepath.Join(tmp, ".env.example"), []byte(envExample), 0644)

	// Source file uses those two plus an undocumented SECRET_TOKEN.
	src := `const db = process.env.DATABASE_URL
const key = process.env.API_KEY
const secret = process.env.SECRET_TOKEN
`
	os.WriteFile(filepath.Join(tmp, "app.ts"), []byte(src), 0644)

	det := EnvConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) == 0 {
		t.Fatal("expected a section for undocumented env var")
	}

	ts, ok := sections[0].(model.TextSection)
	if !ok {
		t.Fatalf("expected TextSection, got %T", sections[0])
	}
	if ts.Category != model.CatSurprise {
		t.Errorf("category = %q, want %q", ts.Category, model.CatSurprise)
	}
	if !strings.Contains(ts.Body, "SECRET_TOKEN") {
		t.Errorf("body should mention SECRET_TOKEN, got: %s", ts.Body)
	}
	// DATABASE_URL and API_KEY are documented, so they shouldn't appear.
	if strings.Contains(ts.Body, "DATABASE_URL") {
		t.Error("body should not mention documented var DATABASE_URL")
	}
}

func TestEnvConventionAllDocumented(t *testing.T) {
	tmp := t.TempDir()

	// .env.example documents everything used in code.
	envExample := "DATABASE_URL=postgres://localhost/dev\nAPI_KEY=your-key-here\n"
	os.WriteFile(filepath.Join(tmp, ".env.example"), []byte(envExample), 0644)

	src := `const db = process.env.DATABASE_URL
const key = process.env.API_KEY
`
	os.WriteFile(filepath.Join(tmp, "app.js"), []byte(src), 0644)

	det := EnvConvention{}
	sections, err := det.Detect(tmp)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(sections) != 0 {
		t.Errorf("expected 0 sections when all vars are documented, got %d", len(sections))
	}
}
