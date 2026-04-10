package extract_yaml_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_yaml"
)

func TestYAMLExtractor_FlatMapping(t *testing.T) {
	raw := []byte(`name: claude-code
version: "1.0"
`)
	cfg := capmon.SelectorConfig{MinResults: 1}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml: %v", err)
	}
	if fv, ok := result.Fields["name"]; !ok || fv.Value != "claude-code" {
		t.Errorf("expected name=claude-code, got %v", result.Fields["name"])
	}
	if fv, ok := result.Fields["version"]; !ok || fv.Value != "1.0" {
		t.Errorf("expected version=1.0, got %v", result.Fields["version"])
	}
}

func TestYAMLExtractor_NestedDotDelimited(t *testing.T) {
	raw := []byte(`hooks:
  PreToolUse: handler
  PostToolUse: other
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml: %v", err)
	}
	if fv, ok := result.Fields["hooks.PreToolUse"]; !ok || fv.Value != "handler" {
		t.Errorf("expected hooks.PreToolUse=handler, got %v", result.Fields)
	}
	if fv, ok := result.Fields["hooks.PostToolUse"]; !ok || fv.Value != "other" {
		t.Errorf("expected hooks.PostToolUse=other, got %v", result.Fields)
	}
}

func TestYAMLExtractor_SequenceValues(t *testing.T) {
	raw := []byte(`events:
  - PreToolUse
  - PostToolUse
  - Stop
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml: %v", err)
	}
	if fv, ok := result.Fields["events.0"]; !ok || fv.Value != "PreToolUse" {
		t.Errorf("expected events.0=PreToolUse, got %v", result.Fields)
	}
	if fv, ok := result.Fields["events.2"]; !ok || fv.Value != "Stop" {
		t.Errorf("expected events.2=Stop, got %v", result.Fields)
	}
}

func TestYAMLExtractor_TopLevelLandmarks(t *testing.T) {
	raw := []byte(`hooks: {}
events: {}
tools: {}
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml: %v", err)
	}
	wantLandmarks := []string{"hooks", "events", "tools"}
	for _, want := range wantLandmarks {
		found := false
		for _, got := range result.Landmarks {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("landmark %q not found in %v", want, result.Landmarks)
		}
	}
}

func TestYAMLExtractor_NoTypeCoercion(t *testing.T) {
	// YAML would normally coerce "true" to bool and "42" to int.
	// The yaml.Node approach preserves raw string values.
	raw := []byte(`enabled: true
count: 42
ratio: 3.14
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract yaml: %v", err)
	}
	// Values should be the raw YAML scalar strings
	if fv, ok := result.Fields["enabled"]; !ok || fv.Value != "true" {
		t.Errorf("expected enabled=true (string), got %v", result.Fields["enabled"])
	}
	if fv, ok := result.Fields["count"]; !ok || fv.Value != "42" {
		t.Errorf("expected count=42 (string), got %v", result.Fields["count"])
	}
}

func TestYAMLExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`key: value
`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}

func TestYAMLExtractor_ParseError(t *testing.T) {
	raw := []byte("key: :\n  - bad: [unclosed")
	cfg := capmon.SelectorConfig{}
	_, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err == nil {
		t.Error("expected parse error for invalid YAML")
	}
}

func TestYAMLExtractor_Partial(t *testing.T) {
	raw := []byte(`only: one
`)
	cfg := capmon.SelectorConfig{MinResults: 10}
	result, err := capmon.Extract(context.Background(), "yaml", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be Partial when below MinResults")
	}
}
