package extract_json_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_json"
)

func TestJSONExtractor_FlatObject(t *testing.T) {
	raw := []byte(`{"name": "claude-code", "version": "1.0"}`)
	cfg := capmon.SelectorConfig{MinResults: 1}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json: %v", err)
	}
	wantFields := map[string]string{
		"name":    "claude-code",
		"version": "1.0",
	}
	for key, want := range wantFields {
		fv, ok := result.Fields[key]
		if !ok {
			t.Errorf("expected field %q", key)
			continue
		}
		if fv.Value != want {
			t.Errorf("field %q: got %q, want %q", key, fv.Value, want)
		}
	}
}

func TestJSONExtractor_NestedDotDelimited(t *testing.T) {
	raw := []byte(`{"hooks": {"PreToolUse": "handler", "PostToolUse": "other"}}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json: %v", err)
	}
	if fv, ok := result.Fields["hooks.PreToolUse"]; !ok || fv.Value != "handler" {
		t.Errorf("expected field hooks.PreToolUse=handler, got %v", result.Fields)
	}
	if fv, ok := result.Fields["hooks.PostToolUse"]; !ok || fv.Value != "other" {
		t.Errorf("expected field hooks.PostToolUse=other, got %v", result.Fields)
	}
}

func TestJSONExtractor_ArrayValues(t *testing.T) {
	raw := []byte(`["PreToolUse", "PostToolUse", "Stop"]`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json: %v", err)
	}
	if fv, ok := result.Fields["0"]; !ok || fv.Value != "PreToolUse" {
		t.Errorf("expected field 0=PreToolUse, got %v", result.Fields)
	}
	if fv, ok := result.Fields["2"]; !ok || fv.Value != "Stop" {
		t.Errorf("expected field 2=Stop, got %v", result.Fields)
	}
}

func TestJSONExtractor_TopLevelLandmarks(t *testing.T) {
	raw := []byte(`{"hooks": {}, "events": {}, "tools": {}}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json: %v", err)
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

func TestJSONExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`{"key": "value"}`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}

func TestJSONExtractor_ParseError(t *testing.T) {
	raw := []byte(`{not valid json}`)
	cfg := capmon.SelectorConfig{}
	_, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}

func TestJSONExtractor_Partial(t *testing.T) {
	raw := []byte(`{"only": "one"}`)
	cfg := capmon.SelectorConfig{MinResults: 10}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be Partial when below MinResults")
	}
}

func TestJSONExtractor_BoolAndNumber(t *testing.T) {
	raw := []byte(`{"enabled": true, "count": 42}`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "json", raw, cfg)
	if err != nil {
		t.Fatalf("Extract json: %v", err)
	}
	if fv, ok := result.Fields["enabled"]; !ok || fv.Value != "true" {
		t.Errorf("expected enabled=true, got %v", result.Fields["enabled"])
	}
	if fv, ok := result.Fields["count"]; !ok || fv.Value != "42" {
		t.Errorf("expected count=42, got %v", result.Fields["count"])
	}
}
