package converter

import (
	"errors"
	"strings"
	"testing"
)

func manifestHook(event string) []byte {
	return []byte(`{"spec":"hooks/0.1","hooks":[{"event":"` + event + `","matcher":"shell","handler":{"type":"command","command":"echo hi"}}]}`)
}

// Regression: convert read hook content only as a provider's own file, so a
// Library hook, stored as a hooks/0.1 manifest, failed to convert at all.
func TestConvertHooks_ReadsEachHookFormat(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		from string
	}{
		{"manifest", string(manifestHook("after_tool_execute")), ""},
		{"legacy hook.json", `{"event":"PostToolUse","matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}`, "claude-code"},
		{"provider file", `{"hooks":{"PostToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hi"}]}]}}`, "claude-code"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ConvertHooks([]byte(tc.raw), tc.from, "gemini-cli")
			if err != nil {
				t.Fatalf("ConvertHooks: %v", err)
			}
			for _, want := range []string{`"AfterTool"`, `"run_shell_command"`, `"echo hi"`} {
				if !strings.Contains(string(r.Content), want) {
					t.Errorf("content missing %s:\n%s", want, r.Content)
				}
			}
		})
	}
}

// The conversion writes what install writes: Pi's hooks are a TypeScript
// extension, not JSON.
func TestConvertHooks_UsesTheInstallEncoder(t *testing.T) {
	r, err := ConvertHooks(manifestHook("after_tool_execute"), "", "pi")
	if err != nil {
		t.Fatalf("ConvertHooks: %v", err)
	}
	if r.Filename != "syllago-hooks.ts" || !strings.Contains(string(r.Content), `pi.on("tool_result"`) {
		t.Errorf("pi output = %s %s, want the TypeScript extension", r.Filename, r.Content)
	}
}

// Regression: a provider syllago has no hook writer for reported a
// conversion instead of saying it cannot write hooks there.
func TestConvertHooks_NoEncoder(t *testing.T) {
	if _, err := ConvertHooks(manifestHook("after_tool_execute"), "", "codex"); !errors.Is(err, ErrNoHookEncoder) {
		t.Errorf("err = %v, want ErrNoHookEncoder", err)
	}
}

// A target that holds none of the hooks is not compatible, and says why.
func TestConvertHooks_AllHooksDropped(t *testing.T) {
	r, err := ConvertHooks(manifestHook("worktree_create"), "", "gemini-cli")
	if err != nil {
		t.Fatalf("ConvertHooks: %v", err)
	}
	if r.Content != nil {
		t.Errorf("content = %q, want nil", r.Content)
	}
	if len(r.Warnings) == 0 || !strings.Contains(r.Warnings[0], "worktree_create") {
		t.Errorf("warnings = %v, want the skipped event", r.Warnings)
	}
}

func TestDecodeHooks_ProviderFileNeedsSource(t *testing.T) {
	raw := []byte(`{"hooks":{"PostToolUse":[{"hooks":[{"type":"command","command":"x"}]}]}}`)
	if _, err := DecodeHooks(raw, ""); err == nil {
		t.Error("want an error for a provider file with no source provider")
	}
	if _, err := DecodeHooks(raw, "codex"); err == nil {
		t.Error("want an error for a source syllago cannot read")
	}
}

// A manifest written with a provider's own event name is read back to the
// canonical event through its source provider.
func TestDecodeHooks_ManifestWithProviderEvent(t *testing.T) {
	h, err := DecodeHooks(manifestHook("PostToolUse"), "claude-code")
	if err != nil {
		t.Fatalf("DecodeHooks: %v", err)
	}
	if h.Hooks[0].Event != "after_tool_execute" {
		t.Errorf("event = %q, want after_tool_execute", h.Hooks[0].Event)
	}
}
