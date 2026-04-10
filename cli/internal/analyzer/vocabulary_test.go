package analyzer

import "testing"

func TestKnownHookEventNames_NotEmpty(t *testing.T) {
	t.Parallel()
	if len(knownHookEventNames) == 0 {
		t.Fatal("knownHookEventNames must not be empty")
	}
	required := []string{"PreToolUse", "PostToolUse", "SessionStart", "before_tool_execute"}
	for _, name := range required {
		if !knownHookEventNames[name] {
			t.Errorf("knownHookEventNames missing required entry %q", name)
		}
	}
}

func TestDirectoryKeywords_NotEmpty(t *testing.T) {
	t.Parallel()
	if len(directoryKeywords) == 0 {
		t.Fatal("directoryKeywords must not be empty")
	}
}

func TestContentSignalExtensions_NotEmpty(t *testing.T) {
	t.Parallel()
	if len(contentSignalExtensions) == 0 {
		t.Fatal("contentSignalExtensions must not be empty")
	}
	required := []string{".md", ".yaml", ".yml", ".json", ".toml"}
	for _, ext := range required {
		if !contentSignalExtensions[ext] {
			t.Errorf("contentSignalExtensions missing %q", ext)
		}
	}
}

func TestKnownHookEventNames_CrossProvider(t *testing.T) {
	t.Parallel()
	providers := map[string][]string{
		"claude-code": {"PreToolUse", "PostToolUse", "SessionStart", "SessionEnd"},
		"gemini":      {"BeforeTool", "AfterTool", "BeforeAgent", "AfterAgent"},
		"windsurf":    {"pre_user_prompt", "post_cascade_response"},
		"copilot":     {"preToolUse", "postToolUse", "sessionStart"},
		"opencode":    {"tool.execute.before", "tool.execute.after"},
		"cursor":      {"beforeAgentResponse", "afterAgentResponse"},
		"canonical":   {"before_tool_execute", "after_tool_execute", "session_start", "session_end"},
	}
	for provider, events := range providers {
		for _, event := range events {
			if !knownHookEventNames[event] {
				t.Errorf("provider %q: missing event %q", provider, event)
			}
		}
	}
}
