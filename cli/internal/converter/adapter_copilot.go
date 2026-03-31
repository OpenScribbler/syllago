package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&CopilotCLIAdapter{})
}

// CopilotCLIAdapter handles hooks for GitHub Copilot CLI.
type CopilotCLIAdapter struct{}

func (a *CopilotCLIAdapter) ProviderSlug() string { return "copilot-cli" }

// --- Copilot provider-native structs ---
// These use the copilotNative* prefix to avoid collision with the legacy
// copilotHookEntry/copilotMatcherGroup/copilotHooksConfig in hooks.go,
// which are kept alive until the legacy bridge is removed in Phase 7.

// copilotNativeEntry is a single hook in Copilot CLI's native format.
// Copilot uses `bash`/`powershell` instead of a single `command` field,
// `timeoutSec` in seconds (matches canonical), and `comment` for status.
type copilotNativeEntry struct {
	Type       string            `json:"type,omitempty"`
	Bash       string            `json:"bash,omitempty"`
	PowerShell string            `json:"powershell,omitempty"`
	TimeoutSec int               `json:"timeoutSec,omitempty"`
	Comment    string            `json:"comment,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Cwd        string            `json:"cwd,omitempty"`
}

// copilotNativeGroup is a matcher group in Copilot CLI's hook format.
type copilotNativeGroup struct {
	Matcher string               `json:"matcher,omitempty"`
	Hooks   []copilotNativeEntry `json:"hooks"`
}

// copilotNativeConfig is the top-level Copilot hooks structure.
type copilotNativeConfig struct {
	Version int                             `json:"version"`
	Hooks   map[string][]copilotNativeGroup `json:"hooks"`
}

func (a *CopilotCLIAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning

	// Keyed grouping: same event+matcher go in the same group
	type gKey struct{ event, matcher string }
	groups := map[gKey]*copilotNativeGroup{}
	var order []gKey

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, "copilot-cli")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by copilot-cli; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type (Copilot only supports command hooks)
		_, hWarnings, keep := TranslateHandlerType(hook.Handler, "copilot-cli", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher
		// Copilot's JSON format supports matchers even though Capabilities() says
		// SupportsMatchers: false (the capability flag reflects filtering behavior,
		// not JSON schema support). We translate the matcher for the JSON output.
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "copilot-cli")
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> Copilot seconds, no conversion)
		timeoutSec := TranslateTimeoutToProvider(hook.Handler.Timeout, "copilot-cli")

		// 5. Build entry
		entry := copilotNativeEntry{
			Type:       "command",
			Bash:       hook.Handler.Command,
			TimeoutSec: timeoutSec,
			Comment:    hook.Handler.StatusMessage,
			Cwd:        hook.Handler.CWD,
			Env:        hook.Handler.Env,
		}

		// 6. Group by event+matcher
		k := gKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &copilotNativeGroup{Matcher: matcherStr, Hooks: []copilotNativeEntry{entry}}
			groups[k] = g
			order = append(order, k)
		}
	}

	// Build result from ordered groups
	result := copilotNativeConfig{
		Version: 1,
		Hooks:   make(map[string][]copilotNativeGroup),
	}
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

func (a *CopilotCLIAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file copilotNativeConfig
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing copilot-cli hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, groups := range file.Hooks {
		// Translate event (decode path: lenient)
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "copilot-cli")

		for _, group := range groups {
			// Translate matcher
			var matcherJSON json.RawMessage
			if group.Matcher != "" {
				rawMatcher, _ := json.Marshal(group.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "copilot-cli")
				matcherJSON = translatedMatcher
			}

			for _, entry := range group.Hooks {
				// Copilot uses bash/powershell fields; prefer bash, fall back to powershell
				cmd := entry.Bash
				if cmd == "" {
					cmd = entry.PowerShell
				}

				// Translate timeout (Copilot seconds -> canonical seconds, no conversion)
				timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutSec, "copilot-cli")

				hook := CanonicalHook{
					Event:   canonEvent,
					Matcher: matcherJSON,
					Handler: HookHandler{
						Type:          "command",
						Command:       cmd,
						Timeout:       timeoutSec,
						StatusMessage: entry.Comment,
						CWD:           entry.Cwd,
						Env:           entry.Env,
					},
				}
				ch.Hooks = append(ch.Hooks, hook)
			}
		}
	}

	return ch, nil
}

func (a *CopilotCLIAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end",
			"subagent_stop", "error_occurred", "tool_use_failure",
		},
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
	}
}
