package converter

import "encoding/json"

func init() {
	RegisterAdapter(&KiroAdapter{})
}

// KiroAdapter handles hooks for Kiro.
type KiroAdapter struct{}

func (a *KiroAdapter) ProviderSlug() string { return "kiro" }

func (a *KiroAdapter) Decode(content []byte) (*CanonicalHooks, error) {
	conv := &HooksConverter{}
	result, err := conv.Canonicalize(content, "kiro")
	if err != nil {
		return nil, err
	}
	var cfg hooksConfig
	if err := json.Unmarshal(result.Content, &cfg); err != nil {
		return nil, err
	}
	return FromLegacyHooksConfig(cfg), nil
}

func (a *KiroAdapter) Encode(hooks *CanonicalHooks) (*EncodedResult, error) {
	cfg := hooks.ToLegacyHooksConfig()
	content, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}
	conv := &HooksConverter{}
	result, err := conv.Render(content, providerBySlug("kiro"))
	if err != nil {
		return nil, err
	}
	return legacyResultToEncoded(result), nil
}

func (a *KiroAdapter) Capabilities() ProviderCapabilities {
	return ProviderCapabilities{
		Events: []string{
			"before_tool_execute", "after_tool_execute", "before_prompt",
			"agent_stop", "session_start",
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
