package converter

import "encoding/json"

func init() {
	RegisterAdapter(&CursorAdapter{})
}

// CursorAdapter handles hooks for Cursor.
type CursorAdapter struct{}

func (a *CursorAdapter) ProviderSlug() string { return "cursor" }

func (a *CursorAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	conv := &HooksConverter{}
	result, err := conv.Canonicalize(content, "cursor")
	if err != nil {
		return nil, err
	}
	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		return nil, err
	}
	return FromLegacyHooksConfig(cfg), nil
}

func (a *CursorAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	cfg := hooks.ToLegacyHooksConfig()
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	conv := &HooksConverter{}
	result, err := conv.Render(content, providerBySlug("cursor"))
	if err != nil {
		return nil, err
	}
	return legacyResultToEncoded(result), nil
}

func (a *CursorAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start", "session_end", "before_compact",
			"tool_use_failure", "subagent_start", "subagent_stop",
			"file_changed", "before_model", "after_model", "before_tool_selection",
		},
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
	}
}
