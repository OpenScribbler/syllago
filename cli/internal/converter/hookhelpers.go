package converter

import (
	"encoding/json"
	"fmt"
	"strings"
)

// --- Event translation ---

// TranslateEventToProvider translates a canonical event name to the target provider's
// native name. Returns an error if the event is unknown or unsupported by the target
// provider (encode path is strict — unknown events are programmer errors).
func TranslateEventToProvider(event, slug string) (string, error) {
	translated, supported := TranslateHookEvent(event, slug)
	if !supported {
		return "", fmt.Errorf("event %q not supported by provider %q", event, slug)
	}
	return translated, nil
}

// TranslateEventFromProvider translates a provider-native event name to canonical.
// Unknown events pass through with a warning for forward compatibility (decode path
// is lenient — the source provider may have added new events since our last update).
func TranslateEventFromProvider(event, slug string) (string, []ConversionWarning) {
	canonical := ReverseTranslateHookEvent(event, slug)
	// ReverseTranslateHookEvent returns the input unchanged when no mapping is found
	if canonical == event {
		// Check whether event is actually a known canonical name already
		if _, ok := HookEvents[event]; ok {
			return event, nil
		}
		return event, []ConversionWarning{{
			Severity:    "warning",
			Description: fmt.Sprintf("unknown %s event %q: passed through as-is for forward compatibility", slug, event),
		}}
	}
	return canonical, nil
}

// --- Timeout translation ---

// TranslateTimeoutToProvider converts canonical seconds to the target provider's
// native timeout unit. Most providers use milliseconds; Copilot uses seconds.
func TranslateTimeoutToProvider(seconds int, slug string) int {
	if seconds == 0 {
		return 0
	}
	switch slug {
	case "copilot-cli":
		return seconds // Copilot uses seconds natively
	default:
		return seconds * 1000 // CC, Gemini, Cursor, Kiro all use milliseconds
	}
}

// TranslateTimeoutFromProvider converts a provider-native timeout value to canonical
// seconds. Most providers use milliseconds; Copilot uses seconds.
func TranslateTimeoutFromProvider(value int, slug string) int {
	if value == 0 {
		return 0
	}
	switch slug {
	case "copilot-cli":
		return value // Copilot already in seconds
	default:
		return value / 1000 // CC, Gemini, Cursor, Kiro use milliseconds
	}
}

// --- Matcher translation ---

// canonicalMCPObject is the canonical matcher representation for MCP tools.
type canonicalMCPObject struct {
	MCP struct {
		Server string `json:"server"`
		Tool   string `json:"tool"`
	} `json:"mcp"`
}

// TranslateMatcherToProvider translates a canonical matcher (json.RawMessage) to the
// target provider's format. Handles all 4 canonical matcher shapes:
//   - bare string: tool name translated via TranslateMatcher
//   - {"pattern":"..."}: regex flattened to bare string with lossy-conversion warning
//   - {"mcp":{"server":"...","tool":"..."}}: MCP tool name built for the target provider
//   - array: each element recursively translated
func TranslateMatcherToProvider(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning) {
	if len(matcher) == 0 {
		return nil, nil
	}

	// Try bare string
	var s string
	if json.Unmarshal(matcher, &s) == nil {
		translated := TranslateMatcher(s, slug)
		result, _ := json.Marshal(translated)
		return result, nil
	}

	// Try object (pattern or mcp)
	var obj map[string]json.RawMessage
	if json.Unmarshal(matcher, &obj) == nil {
		// MCP object
		if mcpRaw, ok := obj["mcp"]; ok {
			var mcpDef struct {
				Server string `json:"server"`
				Tool   string `json:"tool"`
			}
			if json.Unmarshal(mcpRaw, &mcpDef) == nil {
				native := TranslateMCPToProvider(mcpDef.Server, mcpDef.Tool, slug)
				result, _ := json.Marshal(native)
				return result, nil
			}
		}

		// Pattern object
		if patternRaw, ok := obj["pattern"]; ok {
			var pattern string
			if json.Unmarshal(patternRaw, &pattern) == nil {
				result, _ := json.Marshal(pattern)
				return result, []ConversionWarning{{
					Severity:    "warning",
					Capability:  "matcher",
					Description: "pattern matcher flattened to bare string; round-trip will treat it as a tool name, not a regex",
				}}
			}
		}

		// Unknown object shape: pass through with warning
		return matcher, []ConversionWarning{{
			Severity:    "warning",
			Capability:  "matcher",
			Description: "unknown matcher object shape; passed through unchanged",
		}}
	}

	// Try array
	var arr []json.RawMessage
	if json.Unmarshal(matcher, &arr) == nil {
		var warnings []ConversionWarning
		translated := make([]json.RawMessage, len(arr))
		for i, elem := range arr {
			t, w := TranslateMatcherToProvider(elem, slug)
			translated[i] = t
			warnings = append(warnings, w...)
		}
		result, _ := json.Marshal(translated)
		return result, warnings
	}

	// Unknown shape: pass through with warning
	return matcher, []ConversionWarning{{
		Severity:    "warning",
		Capability:  "matcher",
		Description: "unrecognized matcher format; passed through unchanged",
	}}
}

// TranslateMatcherFromProvider translates a provider-native matcher to canonical format.
// Provider-native MCP tool name strings are detected and promoted to canonical MCP objects.
func TranslateMatcherFromProvider(matcher json.RawMessage, slug string) (json.RawMessage, []ConversionWarning) {
	if len(matcher) == 0 {
		return nil, nil
	}

	// Try bare string
	var s string
	if json.Unmarshal(matcher, &s) == nil {
		// Check if it's an MCP-format string
		server, tool := parseMCPToolName(s, slug)
		if server != "" {
			// Promote to canonical MCP object
			mcpObj := canonicalMCPObject{}
			mcpObj.MCP.Server = server
			mcpObj.MCP.Tool = tool
			result, _ := json.Marshal(mcpObj)
			return result, nil
		}
		// Regular tool name: reverse translate
		canonical := ReverseTranslateMatcher(s, slug)
		result, _ := json.Marshal(canonical)
		return result, nil
	}

	// Object (already in canonical form or unknown): pass through
	var obj map[string]json.RawMessage
	if json.Unmarshal(matcher, &obj) == nil {
		return matcher, nil
	}

	// Try array: recursively translate each element
	var arr []json.RawMessage
	if json.Unmarshal(matcher, &arr) == nil {
		var warnings []ConversionWarning
		translated := make([]json.RawMessage, len(arr))
		for i, elem := range arr {
			t, w := TranslateMatcherFromProvider(elem, slug)
			translated[i] = t
			warnings = append(warnings, w...)
		}
		result, _ := json.Marshal(translated)
		return result, warnings
	}

	return matcher, nil
}

// --- MCP tool name helpers ---

// TranslateMCPToProvider builds a provider-native MCP tool name from server and tool.
// Delegates to TranslateMCPToolName using claude-code's format as the canonical source.
func TranslateMCPToProvider(server, tool, slug string) string {
	// Build canonical (CC-format) MCP name, then translate to target
	canonical := "mcp__" + server + "__" + tool
	return TranslateMCPToolName(canonical, "claude-code", slug)
}

// TranslateMCPFromProvider parses a provider-native MCP tool name into server and tool.
// Returns ok=false if the name is not in the provider's MCP format.
func TranslateMCPFromProvider(mcpName, slug string) (server, tool string, ok bool) {
	server, tool = parseMCPToolName(mcpName, slug)
	return server, tool, server != ""
}

// --- Handler type translation ---

// Spec-defined default degradation strategies per capability.
// Used when the hook author doesn't specify a strategy in the degradation map.
var defaultDegradation = map[string]string{
	"llm_evaluated":     "exclude",
	"http_handler":      "warn",
	"input_rewrite":     "block",
	"async_execution":   "warn",
	"platform_commands": "warn",
	"custom_env":        "warn",
	"configurable_cwd":  "warn",
}

// degradationStrategy returns the effective strategy for a capability,
// checking the author's map first, then falling back to spec defaults.
func degradationStrategy(degradation map[string]string, capability string) string {
	if degradation != nil {
		if s, ok := degradation[capability]; ok {
			return s
		}
	}
	if s, ok := defaultDegradation[capability]; ok {
		return s
	}
	return "warn" // safe default for unknown capabilities
}

// TranslateHandlerType checks whether a hook handler type is supported by the target
// provider. Command handlers always pass through. Prompt/agent/http handlers are only
// supported by providers that declare the capability. When a capability is missing,
// the hook's degradation map (or spec defaults) determines the behavior:
//   - "block": return error severity warning, keep=false (conversion should fail)
//   - "exclude": drop with warning + suggestion, keep=false
//   - "warn": drop with warning + suggestion, keep=false (full "warn" with wrapper generation is deferred)
//
// Returns (handler, warnings, keep). keep=false means the handler should be dropped.
// Callers must check warnings for Severity="error" to detect "block" degradation.
func TranslateHandlerType(h HookHandler, slug string, degradation map[string]string) (HookHandler, []ConversionWarning, bool) {
	hType := h.Type
	if hType == "" {
		hType = "command"
	}

	// Command handlers are universally supported
	if hType == "command" {
		return h, nil, true
	}

	// Check adapter capabilities for non-command types
	adapter := AdapterFor(slug)
	if adapter == nil {
		warn := ConversionWarning{
			Severity:    "warning",
			Description: fmt.Sprintf("hook type %q is not supported by %s (unknown provider); hook dropped", hType, slug),
		}
		return HookHandler{}, []ConversionWarning{warn}, false
	}

	caps := adapter.Capabilities()

	switch hType {
	case "prompt", "agent":
		if !caps.SupportsLLMHooks {
			return applyDegradation("llm_evaluated", hType, slug, degradation)
		}
		return h, nil, true
	case "http":
		if !caps.SupportsHTTPHooks {
			return applyDegradation("http_handler", hType, slug, degradation)
		}
		return h, nil, true
	default:
		warn := ConversionWarning{
			Severity:    "warning",
			Description: fmt.Sprintf("unknown hook type %q; hook dropped for %s", hType, slug),
			Suggestion:  "Use type \"command\" for maximum cross-provider compatibility",
		}
		return HookHandler{}, []ConversionWarning{warn}, false
	}
}

// applyDegradation implements the degradation strategy for an unsupported capability.
func applyDegradation(capability, hType, slug string, degradation map[string]string) (HookHandler, []ConversionWarning, bool) {
	strategy := degradationStrategy(degradation, capability)

	switch strategy {
	case "block":
		warn := ConversionWarning{
			Severity:   "error",
			Capability: capability,
			Description: fmt.Sprintf("hook type %q requires %s capability; %s does not support it; "+
				"conversion blocked by degradation policy (strategy: block)", hType, capability, slug),
		}
		return HookHandler{}, []ConversionWarning{warn}, false

	case "exclude":
		warn := ConversionWarning{
			Severity:    "warning",
			Capability:  capability,
			Description: fmt.Sprintf("hook type %q is not supported by %s; hook excluded (strategy: exclude)", hType, slug),
			Suggestion:  suggestionForCapability(capability),
		}
		return HookHandler{}, []ConversionWarning{warn}, false

	default: // "warn" — currently behaves like exclude with suggestion; full wrapper generation deferred
		warn := ConversionWarning{
			Severity:    "warning",
			Capability:  capability,
			Description: fmt.Sprintf("hook type %q is not supported by %s; hook dropped with degraded behavior (strategy: warn)", hType, slug),
			Suggestion:  suggestionForCapability(capability),
		}
		return HookHandler{}, []ConversionWarning{warn}, false
	}
}

// suggestionForCapability returns actionable guidance for each capability type.
func suggestionForCapability(capability string) string {
	switch capability {
	case "llm_evaluated":
		return "Convert to a command hook that calls your preferred LLM CLI, " +
			"or set degradation.llm_evaluated to \"block\" to prevent silent degradation"
	case "http_handler":
		return "Convert to a command hook using curl or wget, " +
			"or set degradation.http_handler to \"block\" to prevent silent degradation"
	default:
		return "Use type \"command\" for maximum cross-provider compatibility"
	}
}

// --- LLM wrapper script generation ---

// GenerateLLMWrapperScript creates a shell script that calls the target provider's CLI
// to evaluate an LLM hook. Delegates to the existing generateLLMWrapperScript function,
// bridging from CanonicalHook to the HookEntry it expects.
func GenerateLLMWrapperScript(hook CanonicalHook, slug, event string, idx int) (string, []byte) {
	he := HookEntry{
		Type:    hook.Handler.Type,
		Command: hook.Handler.Command,
		Prompt:  hook.Handler.Prompt,
		Model:   hook.Handler.Model,
		Agent:   hook.Handler.Agent,
	}
	return generateLLMWrapperScript(he, slug, event, idx)
}

// --- Structured output loss ---

// CheckStructuredOutputLoss compares structured output capabilities between source and
// target providers and returns warnings for any fields that will be lost in conversion.
func CheckStructuredOutputLoss(sourceSlug, targetSlug string) []ConversionWarning {
	if sourceSlug == "" {
		return nil
	}
	lost := OutputFieldsLostWarnings(sourceSlug, targetSlug)
	if len(lost) == 0 {
		return nil
	}
	return []ConversionWarning{{
		Severity:   "warning",
		Capability: "structured_output",
		Description: fmt.Sprintf("structured hook output fields [%s] supported by %s but not by %s (hook output will be ignored)",
			strings.Join(lost, ", "), sourceSlug, targetSlug),
	}}
}
