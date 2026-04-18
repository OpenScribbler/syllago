package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// metadataSourceForTest resolves to the cli/internal/metadata/metadata.go path
// from the cmd/syllago test working directory.
const metadataSourceForTest = "../../internal/metadata/metadata.go"

func TestGenyamlschema_EmitsValidJSON(t *testing.T) {
	orig := yamlSchemaSourcePath
	yamlSchemaSourcePath = metadataSourceForTest
	t.Cleanup(func() { yamlSchemaSourcePath = orig })

	raw := captureStdout(t, func() {
		if err := genyamlschemaCmd.RunE(genyamlschemaCmd, nil); err != nil {
			t.Fatalf("_genyamlschema failed: %v", err)
		}
	})

	var manifest YAMLSchemaManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if manifest.Version != "1" {
		t.Errorf("version = %q, want %q", manifest.Version, "1")
	}
	if manifest.Schema == nil {
		t.Fatal("schema is nil")
	}
	if len(manifest.Schema.Fields) == 0 {
		t.Error("schema has no fields")
	}
}

func TestGenyamlschema_IncludesIDField(t *testing.T) {
	orig := yamlSchemaSourcePath
	yamlSchemaSourcePath = metadataSourceForTest
	t.Cleanup(func() { yamlSchemaSourcePath = orig })

	raw := captureStdout(t, func() {
		_ = genyamlschemaCmd.RunE(genyamlschemaCmd, nil)
	})

	var manifest YAMLSchemaManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	found := false
	for _, f := range manifest.Schema.Fields {
		if f.Name == "ID" {
			found = true
			if !f.Required {
				t.Error("ID field: Required = false, want true")
			}
			if f.YAMLKey != "id" {
				t.Errorf("ID field: YAMLKey = %q, want %q", f.YAMLKey, "id")
			}
			break
		}
	}
	if !found {
		t.Error("ID field not present in schema output")
	}
}

func TestGenyamlschema_MissingSource(t *testing.T) {
	orig := yamlSchemaSourcePath
	yamlSchemaSourcePath = filepath.Join("doesnotexist", "metadata.go")
	t.Cleanup(func() { yamlSchemaSourcePath = orig })

	err := genyamlschemaCmd.RunE(genyamlschemaCmd, nil)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
}
