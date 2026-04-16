//go:build cgo

package extract_typescript

import (
	"context"
	"fmt"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("typescript", &tsExtractor{})
}

type tsExtractor struct{}

func (e *tsExtractor) Extract(_ context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in source", cfg.ExpectedContains)
	}

	root, err := sitter.ParseCtx(context.Background(), raw, typescript.GetLanguage())
	if err != nil {
		return nil, fmt.Errorf("parse TypeScript: %w", err)
	}
	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	// Walk top-level statements
	for i := uint32(0); i < root.ChildCount(); i++ {
		child := root.Child(int(i))
		walkTopLevel(child, raw, fields, &landmarks)
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "typescript",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// walkTopLevel processes a top-level AST node, unwrapping export statements.
func walkTopLevel(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue, landmarks *[]string) {
	typ := node.Type()

	// Unwrap export statements to get the inner declaration
	if typ == "export_statement" {
		for i := uint32(0); i < node.ChildCount(); i++ {
			ch := node.Child(int(i))
			t := ch.Type()
			if t == "enum_declaration" || t == "lexical_declaration" ||
				t == "interface_declaration" || t == "type_alias_declaration" {
				walkTopLevel(ch, src, fields, landmarks)
			}
		}
		return
	}

	switch typ {
	case "enum_declaration":
		extractEnum(node, src, fields, landmarks)
	case "lexical_declaration":
		extractLexical(node, src, fields)
	case "interface_declaration":
		extractInterface(node, src, fields, landmarks)
	case "type_alias_declaration":
		if name := namedChild(node, "name", src); name != "" {
			*landmarks = append(*landmarks, name)
		}
	}
}

// extractInterface extracts property names from an interface declaration.
// The interface name is added as a landmark. Properties are keyed as "InterfaceName.PropertyName".
func extractInterface(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue, landmarks *[]string) {
	ifName := namedChild(node, "name", src)
	if ifName != "" {
		*landmarks = append(*landmarks, ifName)
	}

	for i := uint32(0); i < node.ChildCount(); i++ {
		body := node.Child(int(i))
		bodyType := body.Type()
		if bodyType != "object_type" && bodyType != "interface_body" {
			continue
		}
		for j := uint32(0); j < body.ChildCount(); j++ {
			prop := body.Child(int(j))
			if prop.Type() != "property_signature" {
				continue
			}
			propName := namedChild(prop, "name", src)
			if propName == "" {
				continue
			}
			key := propName
			if ifName != "" {
				key = ifName + "." + propName
			}
			addField(fields, key, propName)
		}
	}
}

// extractEnum extracts member names and string values from an enum declaration.
// The enum name is added as a landmark. Members are keyed as "EnumName.MemberName".
func extractEnum(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue, landmarks *[]string) {
	enumName := namedChild(node, "name", src)
	if enumName != "" {
		*landmarks = append(*landmarks, enumName)
	}

	// Find enum_body
	for i := uint32(0); i < node.ChildCount(); i++ {
		body := node.Child(int(i))
		if body.Type() != "enum_body" {
			continue
		}
		for j := uint32(0); j < body.ChildCount(); j++ {
			member := body.Child(int(j))
			switch member.Type() {
			case "enum_assignment":
				extractEnumAssignment(member, src, enumName, fields)
			case "property_identifier":
				// bare enum member (no assignment)
				name := member.Content(src)
				if name == "" {
					continue
				}
				key := enumName + "." + name
				if enumName == "" {
					key = name
				}
				addField(fields, key, name)
			}
		}
	}
}

// extractEnumAssignment handles `MemberName = "value"` inside an enum body.
func extractEnumAssignment(node *sitter.Node, src []byte, enumName string, fields map[string]capmon.FieldValue) {
	var memberName, value string
	for i := uint32(0); i < node.ChildCount(); i++ {
		ch := node.Child(int(i))
		switch ch.Type() {
		case "property_identifier":
			memberName = ch.Content(src)
		case "string":
			value = stringContent(ch, src)
		case "number", "true", "false":
			value = ch.Content(src)
		}
	}
	if memberName == "" {
		return
	}
	if value == "" {
		value = memberName // fall back to identifier name
	}
	key := enumName + "." + memberName
	if enumName == "" {
		key = memberName
	}
	addField(fields, key, value)
}

// extractLexical handles `const X = "value"` declarations.
func extractLexical(node *sitter.Node, src []byte, fields map[string]capmon.FieldValue) {
	for i := uint32(0); i < node.ChildCount(); i++ {
		ch := node.Child(int(i))
		if ch.Type() != "variable_declarator" {
			continue
		}
		var varName, value string
		for j := uint32(0); j < ch.ChildCount(); j++ {
			part := ch.Child(int(j))
			switch part.Type() {
			case "identifier":
				varName = part.Content(src)
			case "string":
				value = stringContent(part, src)
			}
		}
		if varName != "" && value != "" {
			addField(fields, varName, value)
		}
	}
}

// namedChild returns the content of the first child with the given field name.
func namedChild(node *sitter.Node, fieldName string, src []byte) string {
	ch := node.ChildByFieldName(fieldName)
	if ch == nil {
		return ""
	}
	return ch.Content(src)
}

// stringContent extracts the raw text between the string delimiters.
func stringContent(node *sitter.Node, src []byte) string {
	for i := uint32(0); i < node.ChildCount(); i++ {
		ch := node.Child(int(i))
		t := ch.Type()
		if t == "string_fragment" || t == "string_content" {
			return ch.Content(src)
		}
	}
	// Fallback: strip surrounding quotes from raw content
	raw := node.Content(src)
	raw = strings.TrimPrefix(raw, `"`)
	raw = strings.TrimSuffix(raw, `"`)
	raw = strings.TrimPrefix(raw, `'`)
	raw = strings.TrimSuffix(raw, `'`)
	return raw
}

// addField sanitizes and records a key→value extraction result.
func addField(fields map[string]capmon.FieldValue, key, value string) {
	sanitized := capmon.SanitizeExtractedString(value)
	fields[key] = capmon.FieldValue{
		Value:     sanitized,
		ValueHash: capmon.SHA256Hex([]byte(sanitized)),
	}
}
