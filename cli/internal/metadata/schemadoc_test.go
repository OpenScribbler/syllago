package metadata

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// metadataSourcePath is the canonical path from this test file's runtime cwd
// (the package dir) to the Go source parsed by the schema doc generator.
const metadataSourcePath = "metadata.go"

func TestBuildSchemaDoc_ParsesMeta(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}

	if doc.StructName != "Meta" {
		t.Errorf("StructName = %q, want %q", doc.StructName, "Meta")
	}
	if doc.FormatVersion != CurrentFormatVersion {
		t.Errorf("FormatVersion = %d, want %d", doc.FormatVersion, CurrentFormatVersion)
	}
	if len(doc.Fields) == 0 {
		t.Fatal("no fields extracted from Meta")
	}
}

func TestBuildSchemaDoc_NestedTypes(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}

	names := make(map[string]bool)
	for _, n := range doc.NestedTypes {
		names[n.Name] = true
	}
	for _, want := range []string{"Dependency", "BundledScriptMeta"} {
		if !names[want] {
			t.Errorf("nested type %q missing from schema doc", want)
		}
	}
}

// TestBuildSchemaDoc_AllFieldsHaveGroup is the drift gate: every Meta field
// must be classified into a group. When adding a new Meta field, add it to
// fieldGroups. This test fails the build otherwise.
func TestBuildSchemaDoc_AllFieldsHaveGroup(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}

	var missing []string
	for _, f := range doc.Fields {
		if f.Group == "" {
			missing = append(missing, f.Name)
		}
	}
	if len(missing) > 0 {
		t.Errorf("Meta fields without group assignment (add to fieldGroups): %v", missing)
	}
}

// TestBuildSchemaDoc_AllFieldsHaveComment verifies every Meta field carries
// a trailing line comment (//...). The comment drives the docs reference
// description, so a missing comment silently produces an empty row.
func TestBuildSchemaDoc_AllFieldsHaveComment(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}

	// Exempt a small set of identity fields where the yaml key is already
	// self-describing and a comment would be redundant boilerplate.
	exempt := map[string]bool{
		"ID":           true,
		"Name":         true,
		"Description":  true,
		"Version":      true,
		"Type":         true,
		"Author":       true,
		"Source":       true,
		"Tags":         true,
		"Hidden":       true,
		"PromotedAt":   true,
		"PRBranch":     true,
		"Dependencies": true,
	}

	var missing []string
	for _, f := range doc.Fields {
		if f.Comment != "" || exempt[f.Name] {
			continue
		}
		missing = append(missing, f.Name)
	}
	if len(missing) > 0 {
		t.Errorf("Meta fields without trailing comment (add a // comment on the Go field): %v", missing)
	}
}

// TestBuildSchemaDoc_EnumFieldsPopulated confirms that fields in fieldEnums
// receive their enum values in the output.
func TestBuildSchemaDoc_EnumFieldsPopulated(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}

	for _, f := range doc.Fields {
		want, expect := fieldEnums[f.Name]
		if !expect {
			continue
		}
		if !reflect.DeepEqual(f.EnumValues, want) {
			t.Errorf("field %s: EnumValues = %v, want %v", f.Name, f.EnumValues, want)
		}
	}
}

// TestBuildSchemaDoc_RequiredDerivedFromOmitempty verifies the required flag
// flips when omitempty is absent. The ID field is required (no omitempty),
// Description is optional (has omitempty).
func TestBuildSchemaDoc_RequiredDerivedFromOmitempty(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}
	byName := make(map[string]FieldDoc)
	for _, f := range doc.Fields {
		byName[f.Name] = f
	}

	if !byName["ID"].Required {
		t.Error("ID: Required = false, want true (no omitempty)")
	}
	if byName["Description"].Required {
		t.Error("Description: Required = true, want false (has omitempty)")
	}
}

// TestBuildSchemaDoc_TypeRendering covers the type → human-friendly mapping.
// Uses declarations that exist in the actual Meta struct so this test won't
// drift: []Dependency, *time.Time, []string, etc.
func TestBuildSchemaDoc_TypeRendering(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}
	byName := make(map[string]FieldDoc)
	for _, f := range doc.Fields {
		byName[f.Name] = f
	}

	tests := []struct {
		field string
		want  string
	}{
		{"ID", "string"},
		{"Hidden", "bool"},
		{"Tags", "list of string"},
		{"Dependencies", "list of Dependency"},
		{"CreatedAt", "time.Time"}, // pointer handled by renderType
		{"Confidence", "float64"},
	}
	for _, tt := range tests {
		f, ok := byName[tt.field]
		if !ok {
			t.Errorf("field %s not present in schema", tt.field)
			continue
		}
		if f.Type != tt.want {
			t.Errorf("field %s: type = %q, want %q", tt.field, f.Type, tt.want)
		}
	}
}

func TestBuildSchemaDoc_YAMLKeyStripsModifiers(t *testing.T) {
	doc, err := BuildSchemaDoc(metadataSourcePath)
	if err != nil {
		t.Fatalf("BuildSchemaDoc: %v", err)
	}
	for _, f := range doc.Fields {
		if strings.Contains(f.YAMLKey, ",") {
			t.Errorf("field %s: yaml_key %q contains modifier", f.Name, f.YAMLKey)
		}
	}
}

func TestBuildSchemaDoc_MissingSource(t *testing.T) {
	_, err := BuildSchemaDocFromSource(filepath.Join("doesnotexist", "metadata.go"))
	if err == nil {
		t.Fatal("expected error for missing source file, got nil")
	}
}
