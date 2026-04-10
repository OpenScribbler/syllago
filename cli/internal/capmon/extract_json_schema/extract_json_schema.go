package extract_json_schema

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("json-schema", &jsonSchemaExtractor{})
}

type jsonSchemaExtractor struct{}

func (e *jsonSchemaExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("parse JSON Schema: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	// Extract from $defs (draft 2019-09+) or definitions (draft-07 and earlier)
	for _, defsKey := range []string{"$defs", "definitions"} {
		if defs, ok := schema[defsKey].(map[string]any); ok {
			for name := range defs {
				landmarks = append(landmarks, name)
			}
			extractDefs(defsKey, defs, fields)
		}
	}

	// Also extract top-level enum values if present
	if enumVals, ok := schema["enum"].([]any); ok {
		for i, v := range enumVals {
			if s, ok := v.(string); ok {
				sanitized := capmon.SanitizeExtractedString(s)
				key := fmt.Sprintf("enum.%d", i)
				fields[key] = capmon.FieldValue{
					Value:     sanitized,
					ValueHash: capmon.SHA256Hex([]byte(sanitized)),
				}
			}
		}
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "json-schema",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// extractDefs walks a definitions/defs block and extracts enum values and property names.
func extractDefs(defsKey string, defs map[string]any, out map[string]capmon.FieldValue) {
	for defName, defVal := range defs {
		defMap, ok := defVal.(map[string]any)
		if !ok {
			continue
		}
		prefix := defsKey + "." + defName

		// Extract enum values from this definition
		if enumVals, ok := defMap["enum"].([]any); ok {
			for i, v := range enumVals {
				if s, ok := v.(string); ok {
					sanitized := capmon.SanitizeExtractedString(s)
					key := fmt.Sprintf("%s.enum.%d", prefix, i)
					out[key] = capmon.FieldValue{
						Value:     sanitized,
						ValueHash: capmon.SHA256Hex([]byte(sanitized)),
					}
				}
			}
		}

		// Extract property names from this definition
		if props, ok := defMap["properties"].(map[string]any); ok {
			for propName := range props {
				sanitized := capmon.SanitizeExtractedString(propName)
				key := fmt.Sprintf("%s.properties.%s", prefix, propName)
				out[key] = capmon.FieldValue{
					Value:     sanitized,
					ValueHash: capmon.SHA256Hex([]byte(sanitized)),
				}
			}
		}
	}
}
