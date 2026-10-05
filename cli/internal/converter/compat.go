package converter

import (
	"slices"
	"sort"
)

// CompatLevel represents the compatibility level of a hook for a target provider.
type CompatLevel int

const (
	CompatFull     CompatLevel = iota // All features translate, no behavioral change
	CompatDegraded                    // Minor features lost, core behavior unchanged
	CompatBroken                      // Hook runs but behavior is fundamentally wrong
	CompatNone                        // Cannot install — event doesn't exist on target
)

// Symbol returns the single-character symbol for display.
func (l CompatLevel) Symbol() string {
	switch l {
	case CompatFull:
		return "✓"
	case CompatDegraded:
		return "~"
	case CompatBroken:
		return "!"
	case CompatNone:
		return "✗"
	}
	return "?"
}

// Label returns the human-readable label.
func (l CompatLevel) Label() string {
	switch l {
	case CompatFull:
		return "Full"
	case CompatDegraded:
		return "Degraded"
	case CompatBroken:
		return "Broken"
	case CompatNone:
		return "None"
	}
	return "Unknown"
}

// HookFeature identifies a specific hook capability that may or may not be
// supported by a given provider.
type HookFeature int

const (
	FeatureMatcher HookFeature = iota
	FeatureAsync
	FeatureStatusMessage
	FeatureLLMHook
	FeatureHTTPHook
	FeatureTimeout // fine-grained (ms) vs coarse (seconds)
)

// featureLoss is what happens to a hook that uses a feature its target
// lacks: how far the hook degrades, and the note that says why. It is the
// same for every provider; which provider lacks what comes from its adapter's
// Capabilities().
var featureLoss = map[HookFeature]struct {
	level CompatLevel
	note  string
}{
	FeatureMatcher:       {CompatBroken, "hook fires on ALL tool calls"},
	FeatureAsync:         {CompatBroken, "hook will block execution"},
	FeatureStatusMessage: {CompatDegraded, "no user-visible status"},
	FeatureLLMHook:       {CompatNone, "no prompt or agent hooks"},
	FeatureHTTPHook:      {CompatNone, "no HTTP hooks"},
}

// supportsFeature reports whether caps covers feature. Every adapter writes
// timeouts, converting to its own unit.
func supportsFeature(caps ProviderCapabilities, feature HookFeature) bool {
	switch feature {
	case FeatureMatcher:
		return caps.SupportsMatchers
	case FeatureAsync:
		return caps.SupportsAsync
	case FeatureStatusMessage:
		return caps.SupportsStatusMessage
	case FeatureLLMHook:
		return caps.SupportsLLMHooks
	case FeatureHTTPHook:
		return caps.SupportsHTTPHooks
	}
	return true
}

// HookProviders returns the slugs of providers that have a hook adapter, in
// name order.
func HookProviders() []string {
	slugs := make([]string, 0, len(adapterRegistry))
	for slug := range adapterRegistry {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

// --- Structured output capabilities ---
//
// Hooks can return structured JSON on stdout to influence behavior.
// Field names use the spec's canonical snake_case vocabulary.
// Provider-native field names (e.g., CC's camelCase "updatedInput") are
// handled by the provider's adapter, not by these canonical constants.

// HookOutputField identifies a structured output field that a hook can return.
type HookOutputField string

const (
	OutputUpdatedInput   HookOutputField = "updated_input"
	OutputSuppressOutput HookOutputField = "suppress_output"
	OutputSystemMessage  HookOutputField = "system_message"
	OutputContext        HookOutputField = "context"
	OutputContinue       HookOutputField = "continue"
	OutputDecision       HookOutputField = "decision"
)

// AllOutputFields lists every structured output field, in documentation order.
var AllOutputFields = []HookOutputField{
	OutputUpdatedInput,
	OutputSuppressOutput,
	OutputSystemMessage,
	OutputContext,
	OutputContinue,
	OutputDecision,
}

// OutputFieldsLostWarnings compares source and target provider structured output
// capabilities and returns warnings for fields the source supports but the target
// does not. Returns nil if no capabilities are lost (or if source has none).
func OutputFieldsLostWarnings(sourceProvider, targetSlug string) []string {
	sourceFields := providerHookCapabilities[sourceProvider].OutputFields
	targetFields := providerHookCapabilities[targetSlug].OutputFields

	var warnings []string
	for _, field := range AllOutputFields {
		if slices.Contains(sourceFields, field) && !slices.Contains(targetFields, field) {
			warnings = append(warnings, string(field))
		}
	}
	return warnings
}

// FeatureResult describes what happens to one feature when targeting a provider.
type FeatureResult struct {
	Feature   HookFeature
	Present   bool        // true if the source hook uses this feature
	Supported bool        // true if the target provider supports this feature
	Impact    CompatLevel // impact level when unsupported
	Notes     string
}

// CompatResult is the output of AnalyzeHookCompat for one hook + one provider.
type CompatResult struct {
	Provider string
	Level    CompatLevel     // worst level across all features + event support
	Notes    string          // short summary note
	Features []FeatureResult // per-feature breakdown, only features present in source hook
}

// AnalyzeHookCompat computes compatibility for a single hook against a target provider.
// Checks: (1) event support, (2) per-feature support, (3) aggregates to worst level.
func AnalyzeHookCompat(hook HookData, targetProvider string) CompatResult {
	result := CompatResult{
		Provider: targetProvider,
		Level:    CompatFull,
	}

	// 1. Check event support
	if _, supported := TranslateHookEvent(hook.Event, targetProvider); !supported {
		result.Level = CompatNone
		result.Notes = "Event not supported"
		return result
	}
	if targetProvider == "claude-code" {
		result.Notes = "Native format"
	}

	adapter := AdapterFor(targetProvider)
	if adapter == nil {
		result.Level = CompatNone
		result.Notes = "Provider not hook-capable"
		return result
	}
	caps := adapter.Capabilities()

	// 2. Check features present in the source hook
	// LLM hook check
	hasLLM := false
	hasHTTP := false
	hasAsync := false
	hasStatusMessage := false
	hasTimeout := false
	for _, h := range hook.Hooks {
		if h.Type == "prompt" || h.Type == "agent" {
			hasLLM = true
		}
		if h.Type == "http" {
			hasHTTP = true
		}
		if h.Async {
			hasAsync = true
		}
		if h.StatusMessage != "" {
			hasStatusMessage = true
		}
		if h.Timeout > 0 {
			hasTimeout = true
		}
	}

	type featureCheck struct {
		feature HookFeature
		present bool
	}
	checks := []featureCheck{
		{FeatureMatcher, hook.Matcher != ""},
		{FeatureLLMHook, hasLLM},
		{FeatureHTTPHook, hasHTTP},
		{FeatureAsync, hasAsync},
		{FeatureStatusMessage, hasStatusMessage},
		{FeatureTimeout, hasTimeout},
	}

	for _, check := range checks {
		fr := FeatureResult{
			Feature:   check.feature,
			Present:   check.present,
			Supported: supportsFeature(caps, check.feature),
		}
		if check.feature == FeatureMatcher && caps.ExactToolMatchers {
			if _, ok := exactToolNames(hook.Matcher); !ok {
				fr.Supported = false
			}
		}
		if check.feature == FeatureTimeout {
			fr.Notes = caps.TimeoutUnit
		}

		if check.present && !fr.Supported {
			loss := featureLoss[check.feature]
			fr.Impact = loss.level
			fr.Notes = loss.note
			if loss.level > result.Level {
				result.Level = loss.level
			}
		}

		if check.present {
			result.Features = append(result.Features, fr)
		}
	}

	// Generate summary note
	if result.Level == CompatFull && result.Notes == "" {
		if targetProvider != "claude-code" {
			result.Notes = "All features supported"
		}
	} else if result.Level > CompatFull && result.Notes == "" {
		// Summarize what's broken
		for _, fr := range result.Features {
			if fr.Present && !fr.Supported && fr.Impact == result.Level {
				result.Notes = fr.Notes
				break
			}
		}
	}

	return result
}
