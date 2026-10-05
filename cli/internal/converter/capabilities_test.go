package converter

import (
	"encoding/json"
	"slices"
	"testing"
)

// Every event an adapter lists must survive its own encoder, because the
// install gate trusts the list and the encoder is what writes the file.
func TestCapabilities_EveryListedEventEncodes(t *testing.T) {
	for slug, adapter := range adapterRegistry {
		for _, event := range adapter.Capabilities().Events {
			t.Run(slug+"/"+event, func(t *testing.T) {
				hooks := &CanonicalHooks{Spec: SpecVersion, Hooks: []CanonicalHook{{
					Event:   event,
					Handler: HookHandler{Type: "command", Command: "echo hi"},
				}}}
				encoded, err := adapter.Encode(hooks)
				if err != nil {
					t.Fatalf("Encode: %v", err)
				}
				decoded, err := adapter.Decode(encoded.Content)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if len(decoded.Hooks) != 1 {
					t.Fatalf("got %d hooks back, want 1; warnings %v", len(decoded.Hooks), encoded.Warnings)
				}
			})
		}
	}
}

func TestCapabilities_EventsOmitWhatTheEncoderCannotWrite(t *testing.T) {
	tests := []struct{ slug, event string }{
		// VS Code has no SessionEnd, Notification, or error events.
		{"vs-code-copilot", "session_end"},
		{"vs-code-copilot", "notification"},
		{"vs-code-copilot", "error_occurred"},
		{"vs-code-copilot", "tool_use_failure"},
		// Pi has no subagents, and agent_end already means agent_stop.
		{"pi", "subagent_stop"},
	}
	for _, tt := range tests {
		if slices.Contains(AdapterFor(tt.slug).Capabilities().Events, tt.event) {
			t.Errorf("%s lists %s, which its encoder cannot write", tt.slug, tt.event)
		}
	}
}

// TimeoutUnit decides how every adapter converts timeouts, and any value but
// "seconds" converts as milliseconds, so a typo would scale timeouts 1000x.
func TestCapabilities_TimeoutUnitIsKnown(t *testing.T) {
	for slug, adapter := range adapterRegistry {
		if u := adapter.Capabilities().TimeoutUnit; u != "seconds" && u != "milliseconds" {
			t.Errorf("%s: TimeoutUnit %q, want seconds or milliseconds", slug, u)
		}
	}
}

// A feature an adapter claims must survive its own encoder, because compat
// reports Full on the strength of the claim.
func TestCapabilities_ClaimedFeaturesSurviveEncoding(t *testing.T) {
	features := []struct {
		name    string
		claimed func(ProviderCapabilities) bool
		hook    CanonicalHook
		kept    func(CanonicalHook) bool
	}{
		{
			"matcher", func(c ProviderCapabilities) bool { return c.SupportsMatchers },
			CanonicalHook{Matcher: json.RawMessage(`"shell"`), Handler: HookHandler{Type: "command", Command: "echo hi"}},
			func(h CanonicalHook) bool { return len(h.Matcher) > 0 },
		},
		{
			"async", func(c ProviderCapabilities) bool { return c.SupportsAsync },
			CanonicalHook{Handler: HookHandler{Type: "command", Command: "echo hi", Async: true}},
			func(h CanonicalHook) bool { return h.Handler.Async },
		},
		{
			"status message", func(c ProviderCapabilities) bool { return c.SupportsStatusMessage },
			CanonicalHook{Handler: HookHandler{Type: "command", Command: "echo hi", StatusMessage: "Checking"}},
			func(h CanonicalHook) bool { return h.Handler.StatusMessage == "Checking" },
		},
		{
			"LLM hook", func(c ProviderCapabilities) bool { return c.SupportsLLMHooks },
			CanonicalHook{Handler: HookHandler{Type: "prompt", Prompt: "Is this safe?"}},
			func(h CanonicalHook) bool { return h.Handler.Type == "prompt" },
		},
	}
	for slug, adapter := range adapterRegistry {
		caps := adapter.Capabilities()
		for _, f := range features {
			if !f.claimed(caps) {
				continue
			}
			t.Run(slug+"/"+f.name, func(t *testing.T) {
				hook := f.hook
				hook.Event = "before_tool_execute"
				encoded, err := adapter.Encode(&CanonicalHooks{Spec: SpecVersion, Hooks: []CanonicalHook{hook}})
				if err != nil {
					t.Fatalf("Encode: %v", err)
				}
				decoded, err := adapter.Decode(encoded.Content)
				if err != nil {
					t.Fatalf("Decode: %v", err)
				}
				if len(decoded.Hooks) != 1 || !f.kept(decoded.Hooks[0]) {
					t.Errorf("claims %s but the encoder dropped it; warnings %v", f.name, encoded.Warnings)
				}
			})
		}
	}
}
