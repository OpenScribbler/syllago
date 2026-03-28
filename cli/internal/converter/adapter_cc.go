package converter

import "encoding/json"

func init() {
	RegisterAdapter(&ClaudeCodeAdapter{})
}

// ClaudeCodeAdapter handles hooks for Claude Code.
type ClaudeCodeAdapter struct{}

func (a *ClaudeCodeAdapter) ProviderSlug() string { return "claude-code" }

func (a *ClaudeCodeAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	conv := &HooksConverter{}
	result, err := conv.Canonicalize(content, "claude-code")
	if err != nil {
		return nil, err
	}
	// Parse legacy canonical format
	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		// Try flat format
		var hd HookData
		if err2 := json.Unmarshal(result.Content, &hd); err2 != nil {
			return nil, err
		}
		cfg = hooksConfig{Hooks: map[string][]hookMatcher{
			hd.Event: {{Matcher: hd.Matcher, Hooks: hd.Hooks}},
		}}
	}
	return FromLegacyHooksConfig(cfg), nil
}

func (a *ClaudeCodeAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	cfg := hooks.ToLegacyHooksConfig()
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	conv := &HooksConverter{}
	result, err := conv.Render(content, providerBySlug("claude-code"))
	if err != nil {
		return nil, err
	}
	return legacyResultToEncoded(result), nil
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
		SupportsCWD:              false,
		SupportsEnv:              false,
		SupportsLLMHooks:         true,
		SupportsHTTPHooks:        true,
	}
}
