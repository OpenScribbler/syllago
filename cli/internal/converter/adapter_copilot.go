package converter

import "encoding/json"

func init() {
	RegisterAdapter(&CopilotCLIAdapter{})
}

// CopilotCLIAdapter handles hooks for GitHub Copilot CLI.
type CopilotCLIAdapter struct{}

func (a *CopilotCLIAdapter) ProviderSlug() string { return "copilot-cli" }

func (a *CopilotCLIAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	conv := &HooksConverter{}
	result, err := conv.Canonicalize(content, "copilot-cli")
	if err != nil {
		return nil, err
	}
	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		return nil, err
	}
	return FromLegacyHooksConfig(cfg), nil
}

func (a *CopilotCLIAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	cfg := hooks.ToLegacyHooksConfig()
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	conv := &HooksConverter{}
	result, err := conv.Render(content, providerBySlug("copilot-cli"))
	if err != nil {
		return nil, err
	}
	return legacyResultToEncoded(result), nil
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
