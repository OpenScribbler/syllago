package converter

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
	FeatureTimeout // fine-grained (ms) vs coarse (seconds)
)

// FeatureSupport describes how a provider handles a specific hook feature.
type FeatureSupport struct {
	Supported bool
	Notes     string      // e.g., "mapped to 'comment' field"
	LostLevel CompatLevel // impact level when this feature is used but not supported
}

// ProviderCapability describes what hook features a provider supports.
type ProviderCapability struct {
	Features map[HookFeature]FeatureSupport
}

// HookCapabilities is the single source of truth for provider hook support.
// Used by AnalyzeHookCompat, TUI rendering, and tests.
var HookCapabilities = map[string]ProviderCapability{
	"claude-code": {
		Features: map[HookFeature]FeatureSupport{
			FeatureMatcher:       {Supported: true},
			FeatureAsync:         {Supported: true},
			FeatureStatusMessage: {Supported: true},
			FeatureLLMHook:       {Supported: true},
			FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
		},
	},
	"gemini-cli": {
		Features: map[HookFeature]FeatureSupport{
			FeatureMatcher:       {Supported: true},
			FeatureAsync:         {Supported: true},
			FeatureStatusMessage: {Supported: true},
			FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
			FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
		},
	},
	"copilot-cli": {
		Features: map[HookFeature]FeatureSupport{
			FeatureMatcher:       {Supported: false, LostLevel: CompatBroken, Notes: "hook fires on ALL tool calls"},
			FeatureAsync:         {Supported: false, LostLevel: CompatBroken, Notes: "hook will block execution"},
			FeatureStatusMessage: {Supported: true, Notes: "mapped to 'comment' field"},
			FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
			FeatureTimeout:       {Supported: true, Notes: "converted ms to seconds, precision lost", LostLevel: CompatDegraded},
		},
	},
	"kiro": {
		Features: map[HookFeature]FeatureSupport{
			FeatureMatcher:       {Supported: true, Notes: "per-entry (not group-level)"},
			FeatureAsync:         {Supported: false, LostLevel: CompatBroken, Notes: "hook will block execution"},
			FeatureStatusMessage: {Supported: false, LostLevel: CompatDegraded, Notes: "no user-visible status"},
			FeatureLLMHook:       {Supported: false, LostLevel: CompatNone},
			FeatureTimeout:       {Supported: true, Notes: "milliseconds"},
		},
	},
}

// HookProviders returns the slugs of providers that support hooks, in display order.
func HookProviders() []string {
	return []string{"claude-code", "gemini-cli", "copilot-cli", "kiro"}
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
	if targetProvider == "claude-code" {
		result.Notes = "Native format"
	} else {
		_, supported := TranslateHookEvent(hook.Event, targetProvider)
		if !supported {
			result.Level = CompatNone
			result.Notes = "Event not supported"
			return result
		}
	}

	cap, ok := HookCapabilities[targetProvider]
	if !ok {
		result.Level = CompatNone
		result.Notes = "Provider not hook-capable"
		return result
	}

	// 2. Check features present in the source hook
	// LLM hook check
	hasLLM := false
	hasAsync := false
	hasStatusMessage := false
	hasTimeout := false
	for _, h := range hook.Hooks {
		if h.Type == "prompt" || h.Type == "agent" {
			hasLLM = true
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
		{FeatureAsync, hasAsync},
		{FeatureStatusMessage, hasStatusMessage},
		{FeatureTimeout, hasTimeout},
	}

	for _, check := range checks {
		fs := cap.Features[check.feature]
		fr := FeatureResult{
			Feature:   check.feature,
			Present:   check.present,
			Supported: fs.Supported,
			Notes:     fs.Notes,
		}

		if check.present && !fs.Supported {
			fr.Impact = fs.LostLevel
			if fs.LostLevel > result.Level {
				result.Level = fs.LostLevel
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
