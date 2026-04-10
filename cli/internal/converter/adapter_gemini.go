package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&GeminiCLIAdapter{})
}

// GeminiCLIAdapter handles hooks for Gemini CLI.
type GeminiCLIAdapter struct{}

func (a *GeminiCLIAdapter) ProviderSlug() string { return "gemini-cli" }

// --- Gemini provider-native structs ---

// geminiHookEntry is a single hook in Gemini CLI's native format.
// Gemini uses the same JSON structure as CC, with millisecond timeouts.
// Gemini only supports command hooks — prompt/agent/http types are dropped.
// Gemini natively uses camelCase "statusMessage" in its JSON config (same as CC).
type geminiHookEntry struct {
	Type          string `json:"type,omitempty"`
	Command       string `json:"command,omitempty"`
	TimeoutMs     int    `json:"timeout,omitempty"`       // milliseconds
	StatusMessage string `json:"statusMessage,omitempty"` // camelCase like CC
	Async         bool   `json:"async,omitempty"`
	// Metadata fields stored at entry level for round-trip fidelity
	Name         string            `json:"name,omitempty"`
	Blocking     bool              `json:"blocking,omitempty"`
	Degradation  map[string]string `json:"degradation,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

// geminiMatcherGroup is a matcher group in Gemini CLI's hook format.
type geminiMatcherGroup struct {
	Matcher string            `json:"matcher,omitempty"`
	Hooks   []geminiHookEntry `json:"hooks"`
}

// geminiHooksFile is the top-level Gemini CLI hooks config.
type geminiHooksFile struct {
	Hooks map[string][]geminiMatcherGroup `json:"hooks"`
}

func (a *GeminiCLIAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning

	// Keyed grouping: same event+matcher go in the same group
	type gKey struct{ event, matcher string }
	groups := map[gKey]*geminiMatcherGroup{}
	var order []gKey

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, "gemini-cli")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by gemini-cli; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type (Gemini drops prompt/agent/http)
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "gemini-cli", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "gemini-cli")
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> Gemini milliseconds)
		timeoutMs := TranslateTimeoutToProvider(hook.Handler.Timeout, "gemini-cli")

		// 5. Build entry
		entry := geminiHookEntry{
			Type:          hook.Handler.Type,
			Command:       hook.Handler.Command,
			TimeoutMs:     timeoutMs,
			StatusMessage: hook.Handler.StatusMessage,
			Async:         hook.Handler.Async,
			Name:          hook.Name,
			Blocking:      hook.Blocking,
			Degradation:   hook.Degradation,
			Capabilities:  hook.Capabilities,
		}
		if entry.Type == "" {
			entry.Type = "command"
		}

		// 6. Group by event+matcher
		k := gKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &geminiMatcherGroup{Matcher: matcherStr, Hooks: []geminiHookEntry{entry}}
			groups[k] = g
			order = append(order, k)
		}
	}

	// Build result from ordered groups
	result := geminiHooksFile{Hooks: make(map[string][]geminiMatcherGroup)}
	for _, k := range order {
		result.Hooks[k.event] = append(result.Hooks[k.event], *groups[k])
	}

	content, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "hooks.json",
		Warnings: warnings,
	}, nil
}

func (a *GeminiCLIAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file geminiHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing gemini-cli hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, groups := range file.Hooks {
		// Translate event (decode path: lenient)
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "gemini-cli")

		for _, group := range groups {
			// Translate matcher
			var matcherJSON json.RawMessage
			if group.Matcher != "" {
				rawMatcher, _ := json.Marshal(group.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "gemini-cli")
				matcherJSON = translatedMatcher
			}

			for _, entry := range group.Hooks {
				// Translate timeout (Gemini milliseconds -> canonical seconds)
				timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, "gemini-cli")

				hType := entry.Type
				if hType == "" {
					hType = "command"
				}

				hook := CanonicalHook{
					Name:    entry.Name,
					Event:   canonEvent,
					Matcher: matcherJSON,
					Handler: HookHandler{
						Type:          hType,
						Command:       entry.Command,
						Timeout:       timeoutSec,
						StatusMessage: entry.StatusMessage,
						Async:         entry.Async,
					},
					Blocking:     entry.Blocking,
					Degradation:  entry.Degradation,
					Capabilities: entry.Capabilities,
				}
				ch.Hooks = append(ch.Hooks, hook)
			}
		}
	}

	return ch, nil
}

func (a *GeminiCLIAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"notification", "before_model", "after_model", "before_tool_selection",
		},
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
	}
}
