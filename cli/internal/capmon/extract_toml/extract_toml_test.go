package extract_toml_test

import (
	"context"
	"strings"
	"testing"

	"github.com/OpenScribbler/syllago/cli/internal/capmon"
	_ "github.com/OpenScribbler/syllago/cli/internal/capmon/extract_toml"
)

func TestTOMLExtractor_FlatTable(t *testing.T) {
	raw := []byte(`name = "claude-code"
version = "1.0"
`)
	cfg := capmon.SelectorConfig{MinResults: 1}
	result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract toml: %v", err)
	}
	if fv, ok := result.Fields["name"]; !ok || fv.Value != "claude-code" {
		t.Errorf("expected name=claude-code, got %v", result.Fields["name"])
	}
	if fv, ok := result.Fields["version"]; !ok || fv.Value != "1.0" {
		t.Errorf("expected version=1.0, got %v", result.Fields["version"])
	}
}

func TestTOMLExtractor_NestedDotDelimited(t *testing.T) {
	raw := []byte(`[hooks]
PreToolUse = "handler"
PostToolUse = "other"
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract toml: %v", err)
	}
	if fv, ok := result.Fields["hooks.PreToolUse"]; !ok || fv.Value != "handler" {
		t.Errorf("expected hooks.PreToolUse=handler, got %v", result.Fields)
	}
	if fv, ok := result.Fields["hooks.PostToolUse"]; !ok || fv.Value != "other" {
		t.Errorf("expected hooks.PostToolUse=other, got %v", result.Fields)
	}
}

func TestTOMLExtractor_ArrayOfStrings(t *testing.T) {
	raw := []byte(`events = ["PreToolUse", "PostToolUse", "Stop"]
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract toml: %v", err)
	}
	if fv, ok := result.Fields["events.0"]; !ok || fv.Value != "PreToolUse" {
		t.Errorf("expected events.0=PreToolUse, got %v", result.Fields)
	}
	if fv, ok := result.Fields["events.2"]; !ok || fv.Value != "Stop" {
		t.Errorf("expected events.2=Stop, got %v", result.Fields)
	}
}

func TestTOMLExtractor_TopLevelLandmarks(t *testing.T) {
	raw := []byte(`[hooks]
[events]
[tools]
`)
	cfg := capmon.SelectorConfig{}
	result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err != nil {
		t.Fatalf("Extract toml: %v", err)
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

func TestTOMLExtractor_AnchorMissing(t *testing.T) {
	raw := []byte(`key = "value"
`)
	cfg := capmon.SelectorConfig{ExpectedContains: "NonExistentAnchor"}
	_, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err == nil {
		t.Error("expected error for missing anchor")
	}
	if !strings.Contains(err.Error(), "anchor_missing") {
		t.Errorf("error %q should contain anchor_missing", err.Error())
	}
}

func TestTOMLExtractor_ParseError(t *testing.T) {
	raw := []byte(`this = is not = valid toml!!!`)
	cfg := capmon.SelectorConfig{}
	_, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err == nil {
		t.Error("expected parse error for invalid TOML")
	}
}

func TestTOMLExtractor_Partial(t *testing.T) {
	raw := []byte(`only = "one"
`)
	cfg := capmon.SelectorConfig{MinResults: 10}
	result, err := capmon.Extract(context.Background(), "toml", raw, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Partial {
		t.Error("result should be Partial when below MinResults")
	}
}
