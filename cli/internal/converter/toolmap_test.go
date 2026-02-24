package converter

import (
	"testing"
)

func TestTranslateTool(t *testing.T) {
	tests := []struct {
		name       string
		tool       string
		target     string
		wantResult string
	}{
		{"Read to Gemini", "Read", "gemini-cli", "read_file"},
		{"Read to Copilot", "Read", "copilot-cli", "view"},
		{"Bash to Gemini", "Bash", "gemini-cli", "run_shell_command"},
		{"Unknown tool passes through", "CustomTool", "gemini-cli", "CustomTool"},
		{"WebSearch to Gemini", "WebSearch", "gemini-cli", "google_search"},
		{"WebSearch to Copilot (no mapping)", "WebSearch", "copilot-cli", "WebSearch"},
		{"Claude target returns canonical", "Read", "claude-code", "Read"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateTool(tt.tool, tt.target)
			assertEqual(t, tt.wantResult, got)
		})
	}
}

func TestTranslateTools(t *testing.T) {
	input := []string{"Read", "Write", "Bash"}
	got := TranslateTools(input, "gemini-cli")
	want := []string{"read_file", "write_file", "run_shell_command"}
	if len(got) != len(want) {
		t.Fatalf("expected %d results, got %d", len(want), len(got))
	}
	for i := range want {
		assertEqual(t, want[i], got[i])
	}
}

func TestTranslateHookEvent(t *testing.T) {
	tests := []struct {
		name          string
		event         string
		target        string
		wantTranslate string
		wantSupported bool
	}{
		{"PreToolUse to Gemini", "PreToolUse", "gemini-cli", "BeforeTool", true},
		{"PreToolUse to Copilot", "PreToolUse", "copilot-cli", "preToolUse", true},
		{"Stop to Copilot (unsupported)", "Stop", "copilot-cli", "Stop", false},
		{"SubagentStart (no targets)", "SubagentStart", "gemini-cli", "SubagentStart", false},
		{"Unknown event", "FakeEvent", "gemini-cli", "FakeEvent", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, supported := TranslateHookEvent(tt.event, tt.target)
			assertEqual(t, tt.wantTranslate, got)
			if supported != tt.wantSupported {
				t.Errorf("expected supported=%v, got %v", tt.wantSupported, supported)
			}
		})
	}
}

func TestReverseTranslateHookEvent(t *testing.T) {
	tests := []struct {
		name   string
		event  string
		source string
		want   string
	}{
		{"Gemini BeforeTool", "BeforeTool", "gemini-cli", "PreToolUse"},
		{"Copilot preToolUse", "preToolUse", "copilot-cli", "PreToolUse"},
		{"Unknown event", "FakeEvent", "gemini-cli", "FakeEvent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateHookEvent(tt.event, tt.source)
			assertEqual(t, tt.want, got)
		})
	}
}

func TestReverseTranslateTool(t *testing.T) {
	got := ReverseTranslateTool("read_file", "gemini-cli")
	assertEqual(t, "Read", got)

	got = ReverseTranslateTool("unknown_tool", "gemini-cli")
	assertEqual(t, "unknown_tool", got)
}

func TestTranslateMCPToolName(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		source string
		target string
		want   string
	}{
		{"Claude to Gemini", "mcp__github__search_repositories", "claude-code", "gemini-cli", "github__search_repositories"},
		{"Claude to Copilot", "mcp__github__search_repositories", "claude-code", "copilot-cli", "github/search_repositories"},
		{"Gemini to Claude", "github__search_repositories", "gemini-cli", "claude-code", "mcp__github__search_repositories"},
		{"Copilot to Claude", "github/search_repositories", "copilot-cli", "claude-code", "mcp__github__search_repositories"},
		{"Not MCP tool", "regular_tool", "claude-code", "gemini-cli", "regular_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateMCPToolName(tt.tool, tt.source, tt.target)
			assertEqual(t, tt.want, got)
		})
	}
}
