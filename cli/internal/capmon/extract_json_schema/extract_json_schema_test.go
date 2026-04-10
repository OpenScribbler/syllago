package extract_json_schema_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json_schema"
)

func TestJSONSchemaExtractor_Definitions(t *testing.T) {
	raw := []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "definitions": {
    "HookEvent": {
      "enum": ["PreToolUse", "PostToolUse", "Stop"]
    },
    "ToolName": {
      "enum": ["Bash", "Read", "Write"]
    }
  }
}`)
	cfg := capmon.SelectorConfig{MinResults: 1}
	result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json-schema: %v", err)
	}
	// Landmarks should include definition names
	wantLandmarks := []string{"HookEvent", "ToolName"}
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
	// Enum values should be extracted
	wantValues := []string{"PreToolUse", "PostToolUse", "Stop", "Bash", "Read", "Write"}
	for _, want := range wantValues {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
			}
		}
		if !found {
			t.Errorf("enum value %q not found in fields", want)
		}
	}
}

func TestJSONSchemaExtractor_Defs(t *testing.T) {
	// $defs is the draft 2019-09+ style
	raw := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$defs": {
    "EventType": {
      "enum": ["SessionStart", "SessionEnd"]
    }
  }
}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json-schema: %v", err)
	}
	found := false
	for _, fv := range result.Fields {
		if fv.Value == "SessionStart" {
			found = true
		}
	}
	if !found {
		t.Error("expected SessionStart enum value from $defs")
	}
}

func TestJSONSchemaExtractor_Properties(t *testing.T) {
	raw := []byte(`{
  "definitions": {
    "Config": {
      "properties": {
        "name": {"type": "string"},
        "version": {"type": "string"},
        "enabled": {"type": "boolean"}
      }
    }
  }
}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json-schema: %v", err)
	}
	wantProps := []string{"name", "version", "enabled"}
	for _, want := range wantProps {
		found := false
		for _, fv := range result.Fields {
			if fv.Value == want {
				found = true
			}
		}
		if !found {
			t.Errorf("property name %q not found in fields", want)
		}
	}
}

func TestJSONSchemaExtractor_TopLevelEnum(t *testing.T) {
	raw := []byte(`{
  "enum": ["alpha", "beta", "stable"]
}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json-schema: %v", err)
	}
	if fv, ok := result.Fields["enum.0"]; !ok || fv.Value != "alpha" {
		t.Errorf("expected enum.0=alpha, got %v", result.Fields["enum.0"])
	}
	if fv, ok := result.Fields["enum.2"]; !ok || fv.Value != "stable" {
		t.Errorf("expected enum.2=stable, got %v", result.Fields["enum.2"])
	}
}

func TestJSONSchemaExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`{"$schema": "http://json-schema.org/draft-07/schema#"}`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}

func TestJSONSchemaExtractor_ParseError(t *testing.T) {
	raw := []byte(`{not valid json}`)
	cfg := capmon.SelectorConfig{}
	_, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err == nil {
		t.Error("expected parse error for invalid JSON Schema")
	}
}

func TestJSONSchemaExtractor_Partial(t *testing.T) {
	raw := []byte(`{"enum": ["only"]}`)
	cfg := capmon.SelectorConfig{MinResults: 10}
	result, err := capmon.Extract(context.Background(), "json-schema", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be Partial when below MinResults")
	}
}
