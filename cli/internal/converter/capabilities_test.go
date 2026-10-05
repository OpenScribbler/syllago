package converter

import (
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
