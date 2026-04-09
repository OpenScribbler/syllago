package capmon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func TestGenerateContentTypeViews(t *testing.T) {
	capsDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "by-content-type")

	yamlContent := `schema_version: "1"
slug: test-provider
display_name: Test Provider
content_types:
  hooks:
    supported: true
`
	if err := os.WriteFile(filepath.Join(capsDir, "test-provider.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	if err := capmon.GenerateContentTypeViews(capsDir, outDir); err != nil {
		t.Fatalf("GenerateContentTypeViews: %v", err)
	}

	hooksFile := filepath.Join(outDir, "hooks.yaml")
	data, err := os.ReadFile(hooksFile)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if !strings.Contains(string(data), "THIS FILE IS GENERATED") {
		t.Error("generated file missing THIS FILE IS GENERATED banner")
	}
	if !strings.Contains(string(data), "test-provider") {
		t.Error("generated file missing test-provider entry")
	}
}
