package acif

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// RenderStructured emits the canonical value through a generic structured
// encoder for the framework-level render targets ([ACIF-RENDER] §8,
// TV-RENDER-b). Values are never string-spliced into output.
func RenderStructured(block map[string]any, target string) (*RenderResult, error) {
	switch target {
	case "json-format":
		raw, err := json.Marshal(block)
		if err != nil {
			return nil, err
		}
		return &RenderResult{Output: string(raw)}, nil
	case "yaml-format":
		raw, err := yaml.Marshal(block)
		if err != nil {
			return nil, err
		}
		return &RenderResult{Output: string(raw)}, nil
	default:
		return &RenderResult{Unsupported: true}, nil
	}
}
