package extract_toml

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/OpenScribbler/syllago/cli/internal/capmon"
)

func init() {
	capmon.RegisterExtractor("toml", &tomlExtractor{})
}

type tomlExtractor struct{}

func (e *tomlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
	}

	var root map[string]any
	if _, err := toml.Decode(string(raw), &root); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	for k := range root {
		landmarks = append(landmarks, k)
	}
	flattenMap("", root, fields)

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "toml",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// flattenMap recursively flattens a TOML map into dot-delimited keys.
func flattenMap(prefix string, m map[string]any, out map[string]capmon.FieldValue) {
	for k, v := range m {
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
			flattenMap(key, val, out)
		case []any:
			flattenSlice(key, val, out)
		default:
			s := fmt.Sprintf("%v", val)
			out[key] = capmon.FieldValue{
				Value:     s,
				ValueHash: capmon.SHA256Hex([]byte(s)),
			}
		}
	}
}

// flattenSlice flattens a TOML array into indexed dot-delimited keys.
func flattenSlice(prefix string, arr []any, out map[string]capmon.FieldValue) {
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
			flattenMap(key, val, out)
		case []any:
			flattenSlice(key, val, out)
		default:
			s := fmt.Sprintf("%v", val)
			out[key] = capmon.FieldValue{
				Value:     s,
				ValueHash: capmon.SHA256Hex([]byte(s)),
			}
		}
	}
}
