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
		// OpenCode
		{"Read to OpenCode", "Read", "opencode", "view"},
		{"Write to OpenCode", "Write", "opencode", "write"},
		{"Edit to OpenCode", "Edit", "opencode", "edit"},
		{"Bash to OpenCode", "Bash", "opencode", "bash"},
		{"Glob to OpenCode", "Glob", "opencode", "glob"},
		{"Grep to OpenCode", "Grep", "opencode", "grep"},
		{"WebSearch to OpenCode", "WebSearch", "opencode", "fetch"},
		{"Task to OpenCode", "Task", "opencode", "agent"},
		// Zed
		{"Read to Zed", "Read", "zed", "read_file"},
		{"Write to Zed", "Write", "zed", "edit_file"},
		{"Edit to Zed", "Edit", "zed", "edit_file"},
		{"Bash to Zed", "Bash", "zed", "terminal"},
		{"Glob to Zed", "Glob", "zed", "find_path"},
		{"Grep to Zed", "Grep", "zed", "grep"},
		{"WebSearch to Zed", "WebSearch", "zed", "web_search"},
		{"Task to Zed", "Task", "zed", "subagent"},
		// Cline
		{"Read to Cline", "Read", "cline", "read_file"},
		{"Write to Cline", "Write", "cline", "write_to_file"},
		{"Edit to Cline", "Edit", "cline", "apply_diff"},
		{"Bash to Cline", "Bash", "cline", "execute_command"},
		{"Glob to Cline", "Glob", "cline", "list_files"},
		{"Grep to Cline", "Grep", "cline", "search_files"},
		{"WebSearch to Cline (no mapping)", "WebSearch", "cline", "WebSearch"},
		{"Task to Cline (no mapping)", "Task", "cline", "Task"},
		// Roo Code
		{"Read to RooCode", "Read", "roo-code", "ReadFileTool"},
		{"Write to RooCode", "Write", "roo-code", "WriteToFileTool"},
		{"Edit to RooCode", "Edit", "roo-code", "EditFileTool"},
		{"Bash to RooCode", "Bash", "roo-code", "ExecuteCommandTool"},
		{"Glob to RooCode", "Glob", "roo-code", "ListFilesTool"},
		{"Grep to RooCode", "Grep", "roo-code", "SearchFilesTool"},
		{"WebSearch to RooCode (no mapping)", "WebSearch", "roo-code", "WebSearch"},
		{"Task to RooCode (no mapping)", "Task", "roo-code", "Task"},
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

func TestReverseTranslateTool_NewProviders(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		source string
		want   string
	}{
		// OpenCode
		{"OpenCode view", "view", "opencode", "Read"},
		{"OpenCode write", "write", "opencode", "Write"},
		{"OpenCode edit", "edit", "opencode", "Edit"},
		{"OpenCode bash", "bash", "opencode", "Bash"},
		{"OpenCode glob", "glob", "opencode", "Glob"},
		{"OpenCode grep", "grep", "opencode", "Grep"},
		{"OpenCode fetch", "fetch", "opencode", "WebSearch"},
		{"OpenCode agent", "agent", "opencode", "Task"},
		// Cline
		{"Cline read_file", "read_file", "cline", "Read"},
		{"Cline write_to_file", "write_to_file", "cline", "Write"},
		{"Cline apply_diff", "apply_diff", "cline", "Edit"},
		{"Cline execute_command", "execute_command", "cline", "Bash"},
		{"Cline list_files", "list_files", "cline", "Glob"},
		{"Cline search_files", "search_files", "cline", "Grep"},
		// Roo Code
		{"RooCode ReadFileTool", "ReadFileTool", "roo-code", "Read"},
		{"RooCode WriteToFileTool", "WriteToFileTool", "roo-code", "Write"},
		{"RooCode EditFileTool", "EditFileTool", "roo-code", "Edit"},
		{"RooCode ExecuteCommandTool", "ExecuteCommandTool", "roo-code", "Bash"},
		{"RooCode ListFilesTool", "ListFilesTool", "roo-code", "Glob"},
		{"RooCode SearchFilesTool", "SearchFilesTool", "roo-code", "Grep"},
		// Zed — read_file and grep are unambiguous
		{"Zed read_file", "read_file", "zed", "Read"},
		{"Zed terminal", "terminal", "zed", "Bash"},
		{"Zed find_path", "find_path", "zed", "Glob"},
		{"Zed grep", "grep", "zed", "Grep"},
		{"Zed web_search", "web_search", "zed", "WebSearch"},
		{"Zed subagent", "subagent", "zed", "Task"},
		// Unknown tools pass through
		{"Unknown OpenCode", "unknown_tool", "opencode", "unknown_tool"},
		{"Unknown Cline", "unknown_tool", "cline", "unknown_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateTool(tt.tool, tt.source)
			assertEqual(t, tt.want, got)
		})
	}
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
		// New providers as source
		{"OpenCode to Claude", "github__search_repos", "opencode", "claude-code", "mcp__github__search_repos"},
		{"Cline to Claude", "github__search_repos", "cline", "claude-code", "mcp__github__search_repos"},
		{"RooCode to Claude", "github__search_repos", "roo-code", "claude-code", "mcp__github__search_repos"},
		{"Zed to Claude", "github/search_repos", "zed", "claude-code", "mcp__github__search_repos"},
		// New providers as target
		{"Claude to OpenCode", "mcp__github__search_repos", "claude-code", "opencode", "github__search_repos"},
		{"Claude to Cline", "mcp__github__search_repos", "claude-code", "cline", "github__search_repos"},
		{"Claude to RooCode", "mcp__github__search_repos", "claude-code", "roo-code", "github__search_repos"},
		{"Claude to Zed", "mcp__github__search_repos", "claude-code", "zed", "github/search_repos"},
		// Cross-provider
		{"Gemini to OpenCode", "github__search_repos", "gemini-cli", "opencode", "github__search_repos"},
		{"Copilot to Zed", "github/search_repos", "copilot-cli", "zed", "github/search_repos"},
		{"OpenCode to Zed", "github__search_repos", "opencode", "zed", "github/search_repos"},
		{"Zed to OpenCode", "github/search_repos", "zed", "opencode", "github__search_repos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateMCPToolName(tt.tool, tt.source, tt.target)
			assertEqual(t, tt.want, got)
		})
	}
}
