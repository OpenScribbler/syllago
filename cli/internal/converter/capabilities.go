package converter

import "sort"

// capabilitiesFor returns slug's entry from providerHookCapabilities with
// Events filled from HookEvents, the table the encoders translate through,
// so the install gate never accepts an event the encoder then drops.
func capabilitiesFor(slug string) ProviderCapabilities {
	caps := providerHookCapabilities[slug]
	for canon, native := range HookEvents {
		if _, ok := native[slug]; ok {
			caps.Events = append(caps.Events, canon)
		}
	}
	sort.Strings(caps.Events)
	return caps
}

// providerHookCapabilities is the single data table for hook feature support,
// keyed by provider slug. Each adapter's Capabilities() method returns its
// entry through capabilitiesFor, which adds the events.
var providerHookCapabilities = map[string]ProviderCapabilities{
	"claude-code": {
		SupportsMatchers:      true,
		SupportsAsync:         true,
		SupportsStatusMessage: true,
		OutputFields:          AllOutputFields,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsLLMHooks:      true,
		SupportsHTTPHooks:     true,
	},
	"copilot-cli": {
		SupportsMatchers:      false,
		SupportsAsync:         false,
		SupportsStatusMessage: true,
		OutputFields:          []HookOutputField{OutputDecision}, // preToolUse only
		SupportsBlocking:      true,
		TimeoutUnit:           "seconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"cursor": {
		SupportsMatchers:      true,
		SupportsAsync:         false,
		SupportsStatusMessage: true,
		OutputFields:          []HookOutputField{OutputDecision},
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"gemini-cli": {
		SupportsMatchers:      true,
		SupportsAsync:         true,
		SupportsStatusMessage: true,
		OutputFields:          []HookOutputField{OutputDecision, OutputSystemMessage},
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"kiro": {
		SupportsMatchers:      true,
		SupportsAsync:         false,
		SupportsStatusMessage: false,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"devin": {
		SupportsMatchers: true,
		OutputFields:     []HookOutputField{OutputDecision, OutputUpdatedInput, OutputContext}, // docs.devin.ai/cli/extensibility/hooks
		SupportsBlocking: true,
		TimeoutUnit:      "seconds",
	},
	"vs-code-copilot": {
		SupportsMatchers:      true,
		SupportsAsync:         true,
		SupportsStatusMessage: true,
		OutputFields:          AllOutputFields,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"factory-droid": {
		SupportsMatchers:      true,
		SupportsStatusMessage: true,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
	},
	"crush": {
		// Crush fires hooks only before tool execution (PreToolUse); they run
		// before permission checks with veto power (exit code 2 or a JSON
		// decision of "deny" blocks the tool call).

		SupportsMatchers:      true,
		SupportsAsync:         false,
		SupportsStatusMessage: false,
		OutputFields:          []HookOutputField{OutputDecision, OutputContext, OutputUpdatedInput}, // charmbracelet/crush docs/hooks/README.md
		SupportsBlocking:      true,
		TimeoutUnit:           "seconds",
		SupportsLLMHooks:      false,
		SupportsHTTPHooks:     false,
	},
	"pi": {
		SupportsMatchers:  true,
		ExactToolMatchers: true,
		SupportsBlocking:  true,
		TimeoutUnit:       "milliseconds",
	},
}
