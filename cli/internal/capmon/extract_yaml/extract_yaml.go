package extract_yaml

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	"gopkg.in/yaml.v3"
)

func init() {
	capmon.RegisterExtractor("yaml", &yamlExtractor{})
}

type yamlExtractor struct{}

func (e *yamlExtractor) Extract(ctx context.Context, raw []byte, cfg capmon.SelectorConfig) (*capmon.ExtractedSource, error) {
	if cfg.ExpectedContains != "" && !strings.Contains(string(raw), cfg.ExpectedContains) {
		return nil, fmt.Errorf("anchor_missing: expected_contains %q not found in document", cfg.ExpectedContains)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	fields := make(map[string]capmon.FieldValue)
	var landmarks []string

	// yaml.Unmarshal into yaml.Node produces a document node wrapping the root
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		walkYAMLNode("", root.Content[0], fields, &landmarks, true)
	}

	partial := cfg.MinResults > 0 && len(fields) < cfg.MinResults
	return &capmon.ExtractedSource{
		ExtractorVersion: "1",
		Format:           "yaml",
		ExtractedAt:      time.Now().UTC(),
		Partial:          partial,
		Fields:           fields,
		Landmarks:        landmarks,
	}, nil
}

// walkYAMLNode recursively walks a yaml.Node, flattening keys dot-delimited.
// topLevel is true only for the direct children of a top-level mapping node.
func walkYAMLNode(prefix string, node *yaml.Node, out map[string]capmon.FieldValue, landmarks *[]string, topLevel bool) {
	switch node.Kind {
	case yaml.MappingNode:
		// Mapping nodes store key-value pairs as alternating children
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valNode := node.Content[i+1]
			key := keyNode.Value
			fullKey := key
			if prefix != "" {
				fullKey = prefix + "." + key
			}
			if topLevel {
				*landmarks = append(*landmarks, key)
			}
			walkYAMLNode(fullKey, valNode, out, landmarks, false)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			key := fmt.Sprintf("%d", i)
			if prefix != "" {
				key = prefix + "." + key
			}
			walkYAMLNode(key, child, out, landmarks, false)
		}
	case yaml.ScalarNode:
		if prefix == "" {
			return
		}
		// Use the raw string value — no type coercion
		sanitized := capmon.SanitizeExtractedString(node.Value)
		out[prefix] = capmon.FieldValue{
			Value:     sanitized,
			ValueHash: capmon.SHA256Hex([]byte(sanitized)),
		}
	}
}
