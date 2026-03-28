package converter

import "encoding/json"

func init() {
	RegisterAdapter(&GeminiCLIAdapter{})
}

// GeminiCLIAdapter handles hooks for Gemini CLI.
type GeminiCLIAdapter struct{}

func (a *GeminiCLIAdapter) ProviderSlug() string { return "gemini-cli" }

func (a *GeminiCLIAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	conv := &HooksConverter{}
	result, err := conv.Canonicalize(content, "gemini-cli")
	if err != nil {
		return nil, err
	}
	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		return nil, err
	}
	return FromLegacyHooksConfig(cfg), nil
}

func (a *GeminiCLIAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	cfg := hooks.ToLegacyHooksConfig()
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	conv := &HooksConverter{}
	result, err := conv.Render(content, providerBySlug("gemini-cli"))
	if err != nil {
		return nil, err
	}
	return legacyResultToEncoded(result), nil
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
