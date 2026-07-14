package acif

import "encoding/json"

type RenderResult struct {
	Output      string
	Diagnostics []Diagnostic
	Unsupported bool
	Lossy       []string
}

func RenderHook(block map[string]any, target string, invocation map[string]any) (*RenderResult, error) {
	result, err := CanonicalizeHook(block, HookOpts{})
	if err != nil {
		return nil, err
	}
	handler := firstCommandHandler(result.Canonical)
	switch target {
	case "no-mechanism-provider":
		return renderNoMechanism(result.Canonical, handler, invocation)
	case "per-os-key-map-provider":
		return renderPerOSKeyMapProvider(result.Canonical, handler)
	default:
		return &RenderResult{Unsupported: true}, nil
	}
}

func renderNoMechanism(block map[string]any, handler map[string]any, invocation map[string]any) (*RenderResult, error) {
	defaultScript, constrained := splitDefaultAndConstrained(handler)
	var selected map[string]any
	var diagnostics []Diagnostic
	if defaultScript != nil {
		selected = defaultScript
		if constrained > 0 {
			diagnostics = append(diagnostics, Diagnostic{ID: DiagHookPlatformOverrideDropped})
		}
	} else {
		targetOS, _ := invocation["target_os"].(string)
		if targetOS == "" {
			return nil, hookReject(ErrHookNoDefaultForDegradedRender, "")
		}
		var ok bool
		selected, ok = selectScriptForOS(handler, targetOS)
		if !ok {
			return nil, hookReject(ErrHookNoDefaultForDegradedRender, targetOS)
		}
	}
	if selected == nil || selected["type"] != "file" {
		return &RenderResult{Unsupported: true}, nil
	}
	output := map[string]any{
		"event":   block["event"],
		"command": selected["path"],
	}
	addScriptPassthrough(output, selected)
	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return &RenderResult{Output: string(data), Diagnostics: diagnostics}, nil
}

func renderPerOSKeyMapProvider(_ map[string]any, handler map[string]any) (*RenderResult, error) {
	output := make(map[string]any)
	for _, script := range hookScripts(handler) {
		if script["type"] == "inline" {
			return &RenderResult{Unsupported: true}, nil
		}
		path, _ := script["path"].(string)
		osTags := anyStringSlice(script["os"])
		if len(osTags) == 0 {
			output["command"] = path
		} else {
			for _, osTag := range osTags {
				key := osTag
				if osTag == "darwin" {
					key = "osx"
				}
				output[key] = path
			}
		}
		addScriptPassthrough(output, script)
	}
	data, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return &RenderResult{Output: string(data)}, nil
}

func splitDefaultAndConstrained(handler map[string]any) (map[string]any, int) {
	var defaultScript map[string]any
	constrained := 0
	for _, script := range hookScripts(handler) {
		if _, ok := script["os"]; ok {
			constrained++
		} else {
			defaultScript = script
		}
	}
	return defaultScript, constrained
}

func addScriptPassthrough(output map[string]any, script map[string]any) {
	for key, value := range script {
		switch key {
		case "type", "path", "content", "os", "arch":
			continue
		default:
			output[key] = cloneJSONValue(value)
		}
	}
}
