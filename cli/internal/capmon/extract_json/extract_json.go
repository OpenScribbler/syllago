package extract_json

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("json", &jsonExtractor{})
}

type jsonExtractor struct{}

func (e *jsonExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
	}

	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	switch v := root.(type) {
	case map[string]any:
		for k := range v {
			landmarks = append(landmarks, k)
		}
		flatten("", v, fields)
	case []any:
		flattenArray("", v, fields)
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "json",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// flatten recursively flattens a JSON object into dot-delimited keys.
func flatten(prefix string, obj map[string]any, out map[string]capmon.FieldValue) {
	for k, v := range obj {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			sanitized := capmon.SanitizeExtractedString(val)
			out[key] = capmon.FieldValue{
				Value:     sanitized,
				ValueHash: capmon.SHA256Hex([]byte(sanitized)),
			}
		case map[string]any:
			flatten(key, val, out)
		case []any:
			flattenArray(key, val, out)
		case bool, float64:
			s := fmt.Sprintf("%v", val)
			out[key] = capmon.FieldValue{
				Value:     s,
				ValueHash: capmon.SHA256Hex([]byte(s)),
			}
		}
	}
}

// flattenArray flattens a JSON array into indexed dot-delimited keys.
func flattenArray(prefix string, arr []any, out map[string]capmon.FieldValue) {
	for i, v := range arr {
		key := fmt.Sprintf("%d", i)
		if prefix != "" {
			key = prefix + "." + key
		}
		switch val := v.(type) {
		case string:
			sanitized := capmon.SanitizeExtractedString(val)
			out[key] = capmon.FieldValue{
				Value:     sanitized,
				ValueHash: capmon.SHA256Hex([]byte(sanitized)),
			}
		case map[string]any:
			flatten(key, val, out)
		case []any:
			flattenArray(key, val, out)
		case bool, float64:
			s := fmt.Sprintf("%v", val)
			out[key] = capmon.FieldValue{
				Value:     s,
				ValueHash: capmon.SHA256Hex([]byte(s)),
			}
		}
	}
}
