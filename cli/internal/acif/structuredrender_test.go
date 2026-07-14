package acif

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// TV-RENDER-b: an injection-shaped passthrough value must survive both
// generic structured encoders byte-identically when parsed back.
const injectionValue = "pwsh\", \"payload\": {\"x"

func TestRenderStructuredJSONFormat(t *testing.T) {
	result, err := RenderStructured(map[string]any{"passthrough": injectionValue}, "json-format")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result.Output), &parsed); err != nil {
		t.Fatalf("output does not parse as JSON: %v\noutput: %s", err, result.Output)
	}
	if parsed["passthrough"] != injectionValue {
		t.Fatalf("value did not round-trip: %q", parsed["passthrough"])
	}
}

func TestRenderStructuredYAMLFormat(t *testing.T) {
	result, err := RenderStructured(map[string]any{"passthrough": injectionValue}, "yaml-format")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(result.Output), &parsed); err != nil {
		t.Fatalf("output does not parse as YAML: %v\noutput: %s", err, result.Output)
	}
	if parsed["passthrough"] != injectionValue {
		t.Fatalf("value did not round-trip: %q", parsed["passthrough"])
	}
}

func TestRenderStructuredDeterminism(t *testing.T) {
	block := map[string]any{"b": "2", "a": "1", "nested": map[string]any{"y": true, "x": false}}
	first, err := RenderStructured(block, "json-format")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	second, err := RenderStructured(block, "json-format")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if first.Output != second.Output {
		t.Fatalf("json-format output not deterministic:\n%s\n%s", first.Output, second.Output)
	}
}

func TestRenderStructuredUnknownTargetUnsupported(t *testing.T) {
	result, err := RenderStructured(map[string]any{"passthrough": "x"}, "toml-format")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !result.Unsupported {
		t.Fatal("expected unsupported for unknown target")
	}
}
