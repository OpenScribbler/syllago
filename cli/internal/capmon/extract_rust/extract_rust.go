//go:build cgo

package extract_rust

import (
	"context"
	"fmt"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/rust"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("rust", &rustExtractor{})
}

type rustExtractor struct{}

func (e *rustExtractor) Extract(_ context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in source", cfg.ExpectedContains)
	}

	root, err := sitter.ParseCtx(context.Background(), raw, rust.GetLanguage())
	if err != nil {
		return nil, fmt.Errorf("parse Rust: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(int(i))
		switch child.Type() {
		case "enum_item":
			extractEnum(child, raw, fields, &landmarks)
		case "const_item":
			extractConst(child, raw, fields)
		case "struct_item":
			extractStruct(child, raw, fields, &landmarks)
		case "trait_item", "type_item":
			if name := firstChildOfType(child, "type_identifier", raw); name != "" {
				landmarks = append(landmarks, name)
			}
		}
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "rust",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// extractStruct extracts field names from a struct declaration.
// The struct name is added as a landmark. Fields are keyed as "StructName.field_name".
func extractStruct(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue, landmarks *[]string) {
	structName := firstChildOfType(node, "type_identifier", src)
	if structName != "" {
		*landmarks = append(*landmarks, structName)
	}

	fieldList := firstChildNode(node, "field_declaration_list")
	if fieldList == nil {
		return
	}
	for i := uint32(0); i < fieldList.ChildCount(); i++ {
		decl := fieldList.Child(int(i))
		if decl.Type() != "field_declaration" {
			continue
		}
		fieldName := firstChildOfType(decl, "field_identifier", src)
		if fieldName == "" {
			continue
		}
		key := fieldName
		if structName != "" {
			key = structName + "." + fieldName
		}
		sanitized := capmon.SanitizeExtractedString(fieldName)
		fields[key] = capmon.FieldValue{
			Value:     sanitized,
			ValueHash: capmon.SHA256Hex([]byte(sanitized)),
		}
	}
}

// extractEnum extracts enum name (landmark) and variant names (fields).
// Key format: "EnumName.VariantName", value: variant name string.
func extractEnum(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue, landmarks *[]string) {
	enumName := firstChildOfType(node, "type_identifier", src)
	if enumName != "" {
		*landmarks = append(*landmarks, enumName)
	}

	variantList := firstChildNode(node, "enum_variant_list")
	if variantList == nil {
		return
	}
	for i := uint32(0); i < variantList.ChildCount(); i++ {
		variant := variantList.Child(int(i))
		if variant.Type() != "enum_variant" {
			continue
		}
		variantName := firstChildOfType(variant, "identifier", src)
		if variantName == "" {
			continue
		}
		key := variantName
		if enumName != "" {
			key = enumName + "." + variantName
		}
		sanitized := capmon.SanitizeExtractedString(variantName)
		fields[key] = capmon.FieldValue{
			Value:     sanitized,
			ValueHash: capmon.SHA256Hex([]byte(sanitized)),
		}
	}
}

// extractConst handles `pub const NAME: &str = "value";` declarations.
// Only string literal values are captured; numeric/bool consts are skipped.
func extractConst(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue) {
	name := firstChildOfType(node, "identifier", src)
	if name == "" {
		return
	}
	strLit := firstChildNode(node, "string_literal")
	if strLit == nil {
		return
	}
	value := firstChildOfType(strLit, "string_content", src)
	if value == "" {
		return
	}
	sanitized := capmon.SanitizeExtractedString(value)
	fields[name] = capmon.FieldValue{
		Value:     sanitized,
		ValueHash: capmon.SHA256Hex([]byte(sanitized)),
	}
}

// firstChildOfType returns the Content of the first child with the given node type.
func firstChildOfType(node *sitter.Node, typ string, src []byte) string {
	for i := uint32(0); i < node.ChildCount(); i++ {
		ch := node.Child(int(i))
		if ch.Type() == typ {
			return ch.Content(src)
		}
	}
	return ""
}

// firstChildNode returns the first child with the given node type, or nil.
func firstChildNode(node *sitter.Node, typ string) *sitter.Node {
	for i := uint32(0); i < node.ChildCount(); i++ {
		ch := node.Child(int(i))
		if ch.Type() == typ {
			return ch
		}
	}
	return nil
}
