package converter

import (
	"encoding/json"
	"fmt"
)

func init() {
	RegisterAdapter(&ClaudeCodeAdapter{})
}

// ClaudeCodeAdapter handles hooks for Claude Code.
type ClaudeCodeAdapter struct{}

func (a *ClaudeCodeAdapter) ProviderSlug() string { return "claude-code" }

// --- CC provider-native structs ---

// ccHookEntry is a single hook in Claude Code's native format.
// TimeoutMs is in milliseconds (CC's native unit).
// CC natively uses camelCase "statusMessage" in its JSON config.
type ccHookEntry struct {
	Type          string `json:"type,omitempty"`
	Command       string `json:"command,omitempty"`
	TimeoutMs     int    `json:"timeout,omitempty"`       // milliseconds
	StatusMessage string `json:"statusMessage,omitempty"` // CC uses camelCase
	Async         bool   `json:"async,omitempty"`
	// HTTP fields
	URL            string            `json:"url,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	AllowedEnvVars []string          `json:"allowedEnvVars,omitempty"`
	// Prompt fields
	Prompt string `json:"prompt,omitempty"`
	Model  string `json:"model,omitempty"`
	// Agent fields
	Agent json.RawMessage `json:"agent,omitempty"`
	// Extended canonical fields (passed through to CC's config)
	TimeoutAction string            `json:"timeout_action,omitempty"`
	CWD           string            `json:"cwd,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Platform      map[string]string `json:"platform,omitempty"`
	// Metadata fields stored at entry level for round-trip fidelity
	Name         string            `json:"name,omitempty"`
	Blocking     bool              `json:"blocking,omitempty"`
	Degradation  map[string]string `json:"degradation,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
}

// ccMatcherGroup is a matcher group in Claude Code's hook format.
type ccMatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []ccHookEntry `json:"hooks"`
}

// ccHooksFile is the top-level Claude Code hooks config.
type ccHooksFile struct {
	Hooks map[string][]ccMatcherGroup `json:"hooks"`
}

// groupKey is used to merge hooks with the same event+matcher into one group.
type groupKey struct {
	event   string
	matcher string
}

func (a *ClaudeCodeAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	var warnings []ConversionWarning

	// Keyed grouping: hooks with the same event+matcher go in the same group
	groups := map[groupKey]*ccMatcherGroup{}
	var order []groupKey

	for _, hook := range hooks.Hooks {
		// 1. Translate event
		nativeEvent, err := TranslateEventToProvider(hook.Event, "claude-code")
		if err != nil {
			warnings = append(warnings, ConversionWarning{
				Severity:    "warning",
				Description: fmt.Sprintf("hook event %q not supported by claude-code; skipped", hook.Event),
			})
			continue
		}

		// 2. Check handler type
		handler, hWarnings, keep := TranslateHandlerType(hook.Handler, "claude-code", hook.Degradation)
		warnings = append(warnings, hWarnings...)
		if !keep {
			continue
		}

		// 3. Translate matcher
		var matcherStr string
		if hook.Matcher != nil {
			translatedMatcher, mWarnings := TranslateMatcherToProvider(hook.Matcher, "claude-code")
			warnings = append(warnings, mWarnings...)
			if translatedMatcher != nil {
				_ = json.Unmarshal(translatedMatcher, &matcherStr)
			}
		}

		// 4. Translate timeout (canonical seconds -> CC milliseconds)
		timeoutMs := TranslateTimeoutToProvider(handler.Timeout, "claude-code")

		// 5. Build entry
		entry := ccHookEntry{
			Type:           handler.Type,
			Command:        handler.Command,
			TimeoutMs:      timeoutMs,
			StatusMessage:  handler.StatusMessage,
			Async:          handler.Async,
			URL:            handler.URL,
			Headers:        handler.Headers,
			AllowedEnvVars: handler.AllowedEnvVars,
			Prompt:         handler.Prompt,
			Model:          handler.Model,
			Agent:          handler.Agent,
			TimeoutAction:  handler.TimeoutAction,
			CWD:            handler.CWD,
			Env:            handler.Env,
			Platform:       handler.Platform,
			Name:           hook.Name,
			Blocking:       hook.Blocking,
			Degradation:    hook.Degradation,
			Capabilities:   hook.Capabilities,
		}

		// 6. Group by event+matcher
		k := groupKey{event: nativeEvent, matcher: matcherStr}
		if g, exists := groups[k]; exists {
			g.Hooks = append(g.Hooks, entry)
		} else {
			g := &ccMatcherGroup{Matcher: matcherStr, Hooks: []ccHookEntry{entry}}
			groups[k] = g
			order = append(order, k)
		}
	}

	// Build result from ordered groups
	result := ccHooksFile{Hooks: make(map[string][]ccMatcherGroup)}
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

func (a *ClaudeCodeAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	var file ccHooksFile
	if err := json.Unmarshal(content, &file); err != nil {
		return nil, fmt.Errorf("parsing claude-code hooks: %w", err)
	}

	ch := &CanonicalHooks{Spec: SpecVersion}

	for nativeEvent, groups := range file.Hooks {
		// Translate event (decode path: lenient, warnings discarded)
		canonEvent, _ := TranslateEventFromProvider(nativeEvent, "claude-code")

		for _, group := range groups {
			// Translate matcher
			var matcherJSON json.RawMessage
			if group.Matcher != "" {
				rawMatcher, _ := json.Marshal(group.Matcher)
				translatedMatcher, _ := TranslateMatcherFromProvider(rawMatcher, "claude-code")
				matcherJSON = translatedMatcher
			}

			for _, entry := range group.Hooks {
				// Translate timeout (CC milliseconds -> canonical seconds)
				timeoutSec := TranslateTimeoutFromProvider(entry.TimeoutMs, "claude-code")

				hType := entry.Type
				if hType == "" {
					hType = "command"
				}

				hook := CanonicalHook{
					Name:    entry.Name,
					Event:   canonEvent,
					Matcher: matcherJSON,
					Handler: HookHandler{
						Type:           hType,
						Command:        entry.Command,
						Timeout:        timeoutSec,
						StatusMessage:  entry.StatusMessage,
						Async:          entry.Async,
						URL:            entry.URL,
						Headers:        entry.Headers,
						AllowedEnvVars: entry.AllowedEnvVars,
						Prompt:         entry.Prompt,
						Model:          entry.Model,
						Agent:          entry.Agent,
						TimeoutAction:  entry.TimeoutAction,
						CWD:            entry.CWD,
						Env:            entry.Env,
						Platform:       entry.Platform,
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

func (a *ClaudeCodeAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"notification", "subagent_start", "subagent_stop", "error_occurred",
			"tool_use_failure", "permission_request", "after_compact",
			"instructions_loaded", "config_change", "worktree_create",
			"worktree_remove", "elicitation", "elicitation_result",
			"teammate_idle", "task_completed", "stop_failure", "file_changed",
		},
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
	}
}
