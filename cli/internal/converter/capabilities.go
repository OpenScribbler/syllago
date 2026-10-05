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
		SupportsMatchers:         true,
		SupportsAsync:            true,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         false,
		SupportsCWD:              true,
		SupportsEnv:              true,
		SupportsLLMHooks:         true,
		SupportsHTTPHooks:        true,
	},
	"copilot-cli": {
		SupportsMatchers:         false,
		SupportsAsync:            false,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: false,
		SupportsBlocking:         true,
		TimeoutUnit:              "seconds",
		SupportsPlatform:         false,
		SupportsCWD:              true,
		SupportsEnv:              true,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"cursor": {
		SupportsMatchers:         true,
		SupportsAsync:            false,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         false,
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"gemini-cli": {
		SupportsMatchers:         true,
		SupportsAsync:            true,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         false,
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"kiro": {
		SupportsMatchers:         true,
		SupportsAsync:            false,
		SupportsStatusMessage:    false,
		SupportsStructuredOutput: false,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         false,
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"devin": {
		SupportsMatchers:         true,
		SupportsStructuredOutput: true, // decision, updatedInput, additionalContext
		SupportsBlocking:         true,
		TimeoutUnit:              "seconds",
	},
	"vs-code-copilot": {
		SupportsMatchers:         true,
		SupportsAsync:            true,
		SupportsStatusMessage:    true,
		SupportsStructuredOutput: true,
		SupportsBlocking:         true,
		TimeoutUnit:              "milliseconds",
		SupportsPlatform:         true,
		SupportsCWD:              true,
		SupportsEnv:              true,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"factory-droid": {
		SupportsMatchers:      true,
		SupportsStatusMessage: true,
		SupportsBlocking:      true,
		TimeoutUnit:           "milliseconds",
		SupportsCWD:           true,
		SupportsEnv:           true,
	},
	"crush": {
		// Crush fires hooks only before tool execution (PreToolUse); they run
		// before permission checks with veto power (exit code 2 or a JSON
		// decision of "deny" blocks the tool call).

		SupportsMatchers:         true,
		SupportsAsync:            false,
		SupportsStatusMessage:    false,
		SupportsStructuredOutput: true, // JSON response: decision + context + updated_input
		SupportsBlocking:         true,
		TimeoutUnit:              "seconds",
		SupportsPlatform:         false,
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         false,
		SupportsHTTPHooks:        false,
	},
	"pi": {
		SupportsMatchers: true,
		SupportsBlocking: true,
		TimeoutUnit:      "milliseconds",
	},
}
