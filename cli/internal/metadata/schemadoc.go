package metadata

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
)

// FieldDoc describes a single field on the Meta struct (or a nested type)
// in a form suitable for downstream docs generation.
//
// The comment is the trailing line comment on the Go source field, with the
// leading `//` stripped. The type is a human-friendly rendering rather than
// the raw Go identifier (e.g. `list of string` rather than `[]string`).
type FieldDoc struct {
	Name       string   `json:"name"`
	YAMLKey    string   `json:"yaml_key"`
	Type       string   `json:"type"`
	Omitempty  bool     `json:"omitempty"`
	Required   bool     `json:"required"`
	Comment    string   `json:"comment,omitempty"`
	Group      string   `json:"group,omitempty"`
	EnumValues []string `json:"enum_values,omitempty"`
}

// NestedTypeDoc describes a nested struct referenced by a Meta field
// (currently Dependency and BundledScriptMeta).
type NestedTypeDoc struct {
	Name   string     `json:"name"`
	Fields []FieldDoc `json:"fields"`
}

// SchemaDoc is the top-level schema artifact for .syllago.yaml.
type SchemaDoc struct {
	StructName    string          `json:"struct_name"`
	FormatVersion int             `json:"format_version"`
	Fields        []FieldDoc      `json:"fields"`
	NestedTypes   []NestedTypeDoc `json:"nested_types"`
}

// fieldGroups maps Go field names to their logical group for docs navigation.
// Keep in sync with the groups referenced in syllago-yaml.mdx.
var fieldGroups = map[string]string{
	// identity
	"FormatVersion": "identity",
	"ID":            "identity",
	"Name":          "identity",
	"Description":   "identity",
	"Version":       "identity",
	"Type":          "identity",
	"Author":        "identity",
	"Tags":          "identity",
	"Hidden":        "identity",

	// source-provenance
	"Source":         "source-provenance",
	"SourceProvider": "source-provenance",
	"SourceFormat":   "source-provenance",
	"SourceType":     "source-provenance",
	"SourceURL":      "source-provenance",
	"HasSource":      "source-provenance",
	"SourceHash":     "source-provenance",
	"AddedAt":        "source-provenance",
	"AddedBy":        "source-provenance",

	// registry
	"SourceRegistry":   "registry",
	"SourceVisibility": "registry",

	// scope
	"SourceScope":   "scope",
	"SourceProject": "scope",

	// lifecycle
	"CreatedAt":  "lifecycle",
	"PromotedAt": "lifecycle",
	"PRBranch":   "lifecycle",

	// dependencies
	"Dependencies": "dependencies",

	// bundled-scripts
	"BundledScripts": "bundled-scripts",

	// detection-signals
	"Confidence":      "detection-signals",
	"DetectionSource": "detection-signals",
	"DetectionMethod": "detection-signals",
}

// fieldEnums maps Go field names to their canonical enum values.
// Only fields whose YAML values come from a closed set appear here.
var fieldEnums = map[string][]string{
	"SourceType":       {"git", "filesystem", "registry", "provider"},
	"SourceVisibility": {"public", "private", "unknown"},
	"SourceScope":      {"global", "project"},
	"DetectionMethod":  {"automatic", "user-directed"},
}

// BuildSchemaDoc parses the metadata source file at metadataPath and returns
// a populated SchemaDoc for Meta plus its nested types.
//
// The source file is parsed with go/parser to recover field trailing comments
// (which go/reflect cannot expose). Each field is classified via the package-
// level fieldGroups and fieldEnums maps.
func BuildSchemaDoc(metadataPath string) (*SchemaDoc, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, metadataPath, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", metadataPath, err)
	}

	structs := collectStructs(file)

	metaFields, ok := structs["Meta"]
	if !ok {
		return nil, fmt.Errorf("Meta struct not found in %q", metadataPath)
	}

	doc := &SchemaDoc{
		StructName:    "Meta",
		FormatVersion: CurrentFormatVersion,
		Fields:        buildFields(metaFields),
	}

	nestedNames := []string{"Dependency", "BundledScriptMeta"}
	for _, n := range nestedNames {
		fields, ok := structs[n]
		if !ok {
			continue
		}
		doc.NestedTypes = append(doc.NestedTypes, NestedTypeDoc{
			Name:   n,
			Fields: buildFields(fields),
		})
	}

	return doc, nil
}

// BuildSchemaDocFromSource is a convenience wrapper for tests and generators
// that returns the SchemaDoc for the canonical metadata.go file located at
// path. Returns the error from BuildSchemaDoc unchanged.
func BuildSchemaDocFromSource(path string) (*SchemaDoc, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("schema source: %w", err)
	}
	return BuildSchemaDoc(path)
}

// collectStructs returns a map of struct name to *ast.StructType for every
// type declaration in the file that has a struct type literal.
func collectStructs(file *ast.File) map[string]*ast.StructType {
	out := make(map[string]*ast.StructType)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			out[ts.Name.Name] = st
		}
	}
	return out
}

// buildFields converts an *ast.StructType's fields into []FieldDoc.
func buildFields(st *ast.StructType) []FieldDoc {
	var out []FieldDoc
	for _, f := range st.Fields.List {
		// Skip embedded or tag-less fields; the metadata package does not
		// use them today, so we treat them as schema-invisible.
		if len(f.Names) == 0 || f.Tag == nil {
			continue
		}
		tag := strings.Trim(f.Tag.Value, "`")
		yamlTag := extractYAMLTag(tag)
		if yamlTag == "" {
			continue
		}
		key, omitempty := splitYAMLTag(yamlTag)

		// Use the first name on the field spec (we do not emit grouped fields).
		goName := f.Names[0].Name
		comment := trailingComment(f)
		typeStr := renderType(f.Type)

		fd := FieldDoc{
			Name:       goName,
			YAMLKey:    key,
			Type:       typeStr,
			Omitempty:  omitempty,
			Required:   !omitempty,
			Comment:    comment,
			Group:      fieldGroups[goName],
			EnumValues: fieldEnums[goName],
		}
		out = append(out, fd)
	}
	return out
}

// extractYAMLTag returns the `yaml:"..."` value from a struct tag string,
// or the empty string when no yaml tag is present.
func extractYAMLTag(tag string) string {
	// Naive extraction — tag values in metadata.go do not contain escaped
	// quotes, so strings.Index is sufficient for our source.
	const prefix = `yaml:"`
	i := strings.Index(tag, prefix)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(prefix):]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

// splitYAMLTag separates the key from any modifiers (omitempty, flow, …)
// and returns the key plus a bool indicating whether omitempty is set.
func splitYAMLTag(tag string) (key string, omitempty bool) {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return "", false
	}
	key = parts[0]
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == "omitempty" {
			omitempty = true
		}
	}
	return key, omitempty
}

// trailingComment returns the trailing line comment on a field, cleaned of
// the leading `//` and surrounding whitespace. Returns the empty string when
// the field has no trailing comment.
func trailingComment(f *ast.Field) string {
	if f.Comment == nil {
		return ""
	}
	var parts []string
	for _, c := range f.Comment.List {
		t := strings.TrimPrefix(c.Text, "//")
		parts = append(parts, strings.TrimSpace(t))
	}
	return strings.Join(parts, " ")
}

// renderType turns an AST expression into a human-friendly type string.
// Known shapes: identifiers, pointer types, slice types, map types.
// Unknown shapes fall back to the raw expression.
func renderType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return renderType(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			inner := renderType(t.Elt)
			return "list of " + inner
		}
		return "array"
	case *ast.MapType:
		return "map of " + renderType(t.Key) + " to " + renderType(t.Value)
	case *ast.SelectorExpr:
		// e.g. time.Time
		if pkg, ok := t.X.(*ast.Ident); ok {
			return pkg.Name + "." + t.Sel.Name
		}
		return t.Sel.Name
	case *ast.InterfaceType:
		return "any"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// SortedFieldNames returns the Go field names from a SchemaDoc in declaration
// order. Useful for tests asserting completeness.
func (s *SchemaDoc) SortedFieldNames() []string {
	names := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}
