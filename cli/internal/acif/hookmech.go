package acif

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

func CanonicalizeProviderConfig(config map[string]any, opts HookOpts) (*HookResult, error) {
	provider, _ := config["provider"].(string)
	content, ok := config["content"]
	if !ok {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "provider_config carries no content field; supply the mechanism content")
	}

	decoded, err := decodeProviderContent(content)
	if err != nil {
		return nil, err
	}
	if block, ok := decoded.(map[string]any); ok {
		if _, hasEvent := block["event"]; hasEvent {
			hookOpts := opts
			hookOpts.Provider = provider
			return CanonicalizeHook(block, hookOpts)
		}
	}

	switch provider {
	case "per-os-key-map", "per-os-key-map-provider":
		return canonicalizePerOSKeyMap(decoded, opts)
	case "dual-shell-fields":
		return canonicalizeDualShellFields(decoded, opts)
	case "filename-extension-convention":
		return canonicalizeFilenameExtensionConvention(decoded, opts)
	default:
		return nil, hookReject(ErrHookPlatformUnmappable, provider)
	}
}

func decodeProviderContent(content any) (any, error) {
	if s, ok := content.(string); ok {
		var decoded any
		if err := decodeJSONUseNumberBytes([]byte(s), &decoded); err != nil {
			return nil, hookReject(ErrHookPlatformMechanismMalformed, "content string does not decode as JSON; supply structured mechanism content: "+err.Error())
		}
		return decoded, nil
	}
	return cloneJSONValue(content), nil
}

func canonicalizePerOSKeyMap(content any, opts HookOpts) (*HookResult, error) {
	obj, ok := content.(map[string]any)
	if !ok {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "per-os-key-map content must be an object")
	}

	var scripts []map[string]any
	var defaultEntry map[string]any
	passthrough := make(map[string]any)
	for key, raw := range obj {
		switch key {
		case "command":
			path, ok := raw.(string)
			if !ok {
				return nil, hookReject(ErrHookPlatformMechanismMalformed, "value under \""+key+"\" must be a string entrypoint path")
			}
			defaultEntry = map[string]any{"type": "file", "path": path}
		case "windows", "linux", "osx":
			path, ok := raw.(string)
			if !ok {
				return nil, hookReject(ErrHookPlatformMechanismMalformed, "value under \""+key+"\" must be a string entrypoint path")
			}
			osTag := key
			if osTag == "osx" {
				osTag = "darwin"
			}
			scripts = append(scripts, map[string]any{
				"type": "file",
				"path": path,
				"os":   []any{osTag},
			})
		default:
			passthrough[key] = cloneJSONValue(raw)
		}
	}

	if len(passthrough) > 0 && defaultEntry == nil {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "passthrough keys require a base command entry to carry them; add a command key")
	}
	if defaultEntry != nil {
		for k, v := range passthrough {
			defaultEntry[k] = v
		}
	}

	scripts = mergeConstrainedScriptIdentities(scripts)
	if defaultEntry != nil {
		scripts = append(scripts, defaultEntry)
	}
	return canonicalizeSynthesizedScripts(scripts, "declared", nil, opts)
}

func mergeConstrainedScriptIdentities(scripts []map[string]any) []map[string]any {
	type identity struct {
		typ  string
		path string
	}
	byID := make(map[identity]map[string]any, len(scripts))
	var order []identity
	for _, script := range scripts {
		id := identity{typ: script["type"].(string), path: script["path"].(string)}
		existing, ok := byID[id]
		if !ok {
			copied := cloneJSONValue(script).(map[string]any)
			byID[id] = copied
			order = append(order, id)
			continue
		}
		union := make(map[string]bool)
		for _, osTag := range anyStringSlice(existing["os"]) {
			union[osTag] = true
		}
		for _, osTag := range anyStringSlice(script["os"]) {
			union[osTag] = true
		}
		items := make([]string, 0, len(union))
		for osTag := range union {
			items = append(items, osTag)
		}
		sortStrings(items)
		existing["os"] = stringsToAnySlice(items)
	}

	out := make([]map[string]any, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func canonicalizeDualShellFields(content any, opts HookOpts) (*HookResult, error) {
	obj, ok := content.(map[string]any)
	if !ok {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "dual-shell-fields content must be an object")
	}
	var scripts []map[string]any
	for key, raw := range obj {
		path, ok := raw.(string)
		if !ok {
			return nil, hookReject(ErrHookPlatformMechanismMalformed, "value under \""+key+"\" must be a string entrypoint path")
		}
		switch key {
		case "bash":
			scripts = append(scripts, map[string]any{
				"type": "file",
				"path": path,
				"os":   []any{"darwin", "linux"},
			})
		case "powershell":
			scripts = append(scripts, map[string]any{
				"type": "file",
				"path": path,
				"os":   []any{"windows"},
			})
		default:
			return nil, hookReject(ErrHookPlatformMechanismMalformed, "key \""+key+"\" is not in the closed dual-shell key set {bash, powershell}")
		}
	}
	if len(scripts) == 0 {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "dual-shell-fields requires at least one of bash, powershell")
	}
	return canonicalizeSynthesizedScripts(scripts, "inferred-from-convention", []Diagnostic{{ID: DiagHookPlatformShellOSProxy}}, opts)
}

func canonicalizeFilenameExtensionConvention(content any, opts HookOpts) (*HookResult, error) {
	obj, ok := content.(map[string]any)
	if !ok {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "filename-extension-convention content must be an object")
	}
	path, ok := obj["file"].(string)
	if !ok {
		return nil, hookReject(ErrHookPlatformMechanismMalformed, "filename-extension-convention requires a string file field")
	}

	script := map[string]any{"type": "file", "path": path}
	diagID := DiagHookPlatformFilenameUninferable
	provenance := "declared"
	switch finalHookExtension(filepath.Base(path)) {
	case ".ps1", ".cmd", ".bat":
		script["os"] = []any{"windows"}
		diagID = DiagHookPlatformFilenameInferred
		provenance = "inferred-from-convention"
	case ".sh", "":
		script["os"] = []any{"darwin", "linux"}
		diagID = DiagHookPlatformFilenameInferred
		provenance = "inferred-from-convention"
	}
	return canonicalizeSynthesizedScripts([]map[string]any{script}, provenance, []Diagnostic{{ID: diagID}}, opts)
}

func finalHookExtension(base string) string {
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(base[dot:])
}

func canonicalizeSynthesizedScripts(scripts []map[string]any, provenance string, diagnostics []Diagnostic, opts HookOpts) (*HookResult, error) {
	rawScripts := make([]any, len(scripts))
	for i := range scripts {
		rawScripts[i] = scripts[i]
	}
	block := map[string]any{
		"event": "before_tool_execute",
		"handlers": []any{
			map[string]any{"type": "command", "scripts": rawScripts},
		},
	}
	result, err := CanonicalizeHook(block, HookOpts{BodyRoot: opts.BodyRoot})
	if err != nil {
		return nil, err
	}
	result.Provenance = provenance
	result.Diagnostics = append(result.Diagnostics, diagnostics...)
	return result, nil
}

func decodeJSONUseNumberBytes(data []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	return dec.Decode(v)
}

func sortStrings(items []string) {
	if len(items) < 2 {
		return
	}
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j] < items[j-1]; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
