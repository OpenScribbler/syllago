package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&KiroAdapter{})
}

// KiroAdapter handles hooks for Kiro.
type KiroAdapter struct{}

func (a *KiroAdapter) ProviderSlug() string { return "kiro" }

// --- Kiro provider-native structs ---
// These use the kiroNative* prefix to avoid collision with the legacy
// kiroHookEntry/kiroHooksAgent in hooks.go, which are kept alive
// until the legacy bridge is removed in Phase 7.

// kiroNativeEntry is a single hook in Kiro's native format.
// Kiro uses per-entry matchers (each hook entry has its own matcher,
// not grouped like CC/Gemini/Cursor). Timeout is in milliseconds.
type kiroNativeEntry struct {
	Command   string `json:"command"`
	Matcher   string `json:"matcher,omitempty"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
}

// kiroNativeAgent is the agent-wrapper format Kiro reads.
// Hooks live inside a JSON file with name/description/prompt metadata.
type kiroNativeAgent struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Prompt      string                       `json:"prompt"`
	Hooks       map[string][]kiroNativeEntry `json:"hooks"`
}

// Default agent metadata when encoding to Kiro format.
const (
	kiroDefaultName        = "syllago-hooks"
	kiroDefaultDescription = "Hooks installed by syllago"
)

func (a *KiroAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning

	// Start with default agent metadata; override from provider_data if present
	agentName := kiroDefaultName
	agentDesc := kiroDefaultDescription
	agentPrompt := ""

	if len(hooks.Hooks) > 0 && hooks.Hooks[0].ProviderData != nil {
		if kiroData, ok := hooks.Hooks[0].ProviderData["kiro"].(map[string]any); ok {
			if n, ok := kiroData["name"].(string); ok && n != "" {
				agentName = n
			}
			if d, ok := kiroData["description"].(string); ok && d != "" {
				agentDesc = d
			}
			if p, ok := kiroData["prompt"].(string); ok {
				agentPrompt = p
			}
		}
	}

	kiroHooks := make(map[string][]kiroNativeEntry)

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, "kiro")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by kiro; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type (Kiro only supports command hooks)
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "kiro", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher (Kiro supports per-entry matchers)
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "kiro")
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> Kiro milliseconds)
		timeoutMs := TranslateTimeoutToProvider(hook.Handler.Timeout, "kiro")

		// 5. Build entry (one entry per hook — Kiro is per-entry, not grouped)
		entry := kiroNativeEntry{
			Command:   hook.Handler.Command,
			Matcher:   matcherStr,
			TimeoutMs: timeoutMs,
		}
		kiroHooks[nativeEvent] = append(kiroHooks[nativeEvent], entry)
	}

	agent := kiroNativeAgent{
		Name:        agentName,
		Description: agentDesc,
		Prompt:      agentPrompt,
		Hooks:       kiroHooks,
	}

	content, err := json.MarshalIndent(agent, "", "  ")
	if err != nil {
		return nil, err
	}
	return &EncodedResult{
		Content:  content,
		Filename: "syllago-hooks.json",
		Warnings: warnings,
	}, nil
}

func (a *KiroAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file kiroNativeAgent
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing kiro hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	// Determine if agent metadata is non-default (worth preserving in provider_data)
	var agentProviderData map[string]any
	if file.Name != kiroDefaultName || file.Description != kiroDefaultDescription || file.Prompt != "" {
		agentProviderData = map[string]any{
			"kiro": map[string]any{
				"name":        file.Name,
				"description": file.Description,
				"prompt":      file.Prompt,
			},
		}
	}

	for nativeEvent, entries := range file.Hooks {
		// Translate event (decode path: lenient)
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "kiro")

		for _, entry := range entries {
			// Translate per-entry matcher
			var matcherJSON json.RawMessage
			if entry.Matcher != "" {
				rawMatcher, _ := json.Marshal(entry.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "kiro")
				matcherJSON = translatedMatcher
			}

			// Translate timeout (Kiro milliseconds -> canonical seconds)
			timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, "kiro")

			hook := CanonicalHook{
				Event:   canonEvent,
				Matcher: matcherJSON,
				Handler: HookHandler{
					Type:    "command",
					Command: entry.Command,
					Timeout: timeoutSec,
				},
				ProviderData: agentProviderData,
			}
			ch.Hooks = append(ch.Hooks, hook)
		}
	}

	return ch, nil
}

func (a *KiroAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start",
			"file_changed", "file_created", "file_deleted", "before_task", "after_task",
		},
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
	}
}
