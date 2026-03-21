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
		{"WebSearch to Gemini", "WebSearch", "gemini-cli", "google_web_search"},
		{"WebSearch to Copilot (no mapping)", "WebSearch", "copilot-cli", "WebSearch"},
		{"Claude target returns canonical", "Read", "claude-code", "Read"},
		// OpenCode
		{"Read to OpenCode", "Read", "opencode", "read"},
		{"Write to OpenCode", "Write", "opencode", "write"},
		{"Edit to OpenCode", "Edit", "opencode", "edit"},
		{"Bash to OpenCode", "Bash", "opencode", "bash"},
		{"Glob to OpenCode", "Glob", "opencode", "glob"},
		{"Grep to OpenCode", "Grep", "opencode", "grep"},
		{"WebSearch to OpenCode", "WebSearch", "opencode", "websearch"},
		{"Agent to OpenCode", "Agent", "opencode", "task"},
		// Zed
		{"Read to Zed", "Read", "zed", "read_file"},
		{"Write to Zed", "Write", "zed", "edit_file"},
		{"Edit to Zed", "Edit", "zed", "edit_file"},
		{"Bash to Zed", "Bash", "zed", "terminal"},
		{"Glob to Zed", "Glob", "zed", "find_path"},
		{"Grep to Zed", "Grep", "zed", "grep"},
		{"WebSearch to Zed", "WebSearch", "zed", "web_search"},
		{"Agent to Zed", "Agent", "zed", "spawn_agent"},
		// Cline
		{"Read to Cline", "Read", "cline", "read_file"},
		{"Write to Cline", "Write", "cline", "write_to_file"},
		{"Edit to Cline", "Edit", "cline", "replace_in_file"},
		{"Bash to Cline", "Bash", "cline", "execute_command"},
		{"Glob to Cline", "Glob", "cline", "list_files"},
		{"Grep to Cline", "Grep", "cline", "search_files"},
		{"WebSearch to Cline (no mapping)", "WebSearch", "cline", "WebSearch"},
		{"Agent to Cline (no mapping)", "Agent", "cline", "Agent"},
		// Roo Code
		{"Read to RooCode", "Read", "roo-code", "read_file"},
		{"Write to RooCode", "Write", "roo-code", "write_to_file"},
		{"Edit to RooCode", "Edit", "roo-code", "replace_in_file"},
		{"Bash to RooCode", "Bash", "roo-code", "execute_command"},
		{"Glob to RooCode", "Glob", "roo-code", "list_files"},
		{"Grep to RooCode", "Grep", "roo-code", "search_files"},
		{"WebSearch to RooCode (no mapping)", "WebSearch", "roo-code", "WebSearch"},
		{"Agent to RooCode (no mapping)", "Agent", "roo-code", "Agent"},
		// Cursor
		{"Read to Cursor", "Read", "cursor", "read_file"},
		{"Edit to Cursor", "Edit", "cursor", "edit_file"},
		{"Bash to Cursor", "Bash", "cursor", "run_terminal_cmd"},
		{"Glob to Cursor", "Glob", "cursor", "file_search"},
		{"Grep to Cursor", "Grep", "cursor", "grep_search"},
		{"WebSearch to Cursor", "WebSearch", "cursor", "web_search"},
		// Windsurf
		{"Read to Windsurf", "Read", "windsurf", "view_line_range"},
		{"Write to Windsurf", "Write", "windsurf", "write_to_file"},
		{"Edit to Windsurf", "Edit", "windsurf", "edit_file"},
		{"Bash to Windsurf", "Bash", "windsurf", "run_command"},
		{"Glob to Windsurf", "Glob", "windsurf", "find_by_name"},
		{"Grep to Windsurf", "Grep", "windsurf", "grep_search"},
		{"WebSearch to Windsurf", "WebSearch", "windsurf", "search_web"},
		{"WebFetch to Windsurf", "WebFetch", "windsurf", "read_url_content"},
		// Codex
		{"Read to Codex", "Read", "codex", "read_file"},
		{"Edit to Codex", "Edit", "codex", "apply_patch"},
		{"Bash to Codex", "Bash", "codex", "shell"},
		{"Glob to Codex", "Glob", "codex", "list_dir"},
		{"Grep to Codex", "Grep", "codex", "grep_files"},
		{"WebSearch to Codex", "WebSearch", "codex", "web_search"},
		{"Agent to Codex", "Agent", "codex", "spawn_agent"},
		// Copilot (fixed entries)
		{"Write to Copilot", "Write", "copilot-cli", "create"},
		{"Edit to Copilot", "Edit", "copilot-cli", "edit"},
		{"Grep to Copilot", "Grep", "copilot-cli", "grep"},
		{"Bash to Copilot", "Bash", "copilot-cli", "bash"},
		// Kiro (new entries)
		{"Glob to Kiro", "Glob", "kiro", "glob"},
		{"Grep to Kiro", "Grep", "kiro", "grep"},
		{"WebSearch to Kiro", "WebSearch", "kiro", "web_search"},
		{"Agent to Kiro", "Agent", "kiro", "use_subagent"},
		// WebFetch
		{"WebFetch to Gemini", "WebFetch", "gemini-cli", "web_fetch"},
		{"WebFetch to Copilot", "WebFetch", "copilot-cli", "web_fetch"},
		{"WebFetch to Kiro", "WebFetch", "kiro", "web_fetch"},
		{"WebFetch to OpenCode", "WebFetch", "opencode", "webfetch"},
		{"WebFetch to Zed", "WebFetch", "zed", "fetch"},
		{"WebFetch to Cline (no mapping)", "WebFetch", "cline", "WebFetch"},
		{"WebFetch to RooCode (no mapping)", "WebFetch", "roo-code", "WebFetch"},
		{"WebFetch to Cursor (no mapping)", "WebFetch", "cursor", "WebFetch"},
		{"WebFetch to Codex (no mapping)", "WebFetch", "codex", "WebFetch"},
		// CC-only tools pass through
		{"NotebookEdit to Gemini (no mapping)", "NotebookEdit", "gemini-cli", "NotebookEdit"},
		{"Skill to Gemini (no mapping)", "Skill", "gemini-cli", "Skill"},
		{"AskUserQuestion to Gemini (no mapping)", "AskUserQuestion", "gemini-cli", "AskUserQuestion"},
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
		// Cline hook events
		{"PreToolUse to Cline", "PreToolUse", "cline", "PreToolUse", true},
		{"PostToolUse to Cline", "PostToolUse", "cline", "PostToolUse", true},
		{"SessionStart to Cline", "SessionStart", "cline", "TaskStart", true},
		{"SessionEnd to Cline", "SessionEnd", "cline", "TaskComplete", true},
		{"PreCompact to Cline", "PreCompact", "cline", "PreCompact", true},
		// Cline-only events
		{"TaskResume to Cline", "TaskResume", "cline", "TaskResume", true},
		{"TaskCancel to Cline", "TaskCancel", "cline", "TaskCancel", true},
		// Copilot new events
		{"SubagentStop to Copilot", "SubagentStop", "copilot-cli", "subagentStop", true},
		{"AgentStop to Copilot", "AgentStop", "copilot-cli", "agentStop", true},
		{"ErrorOccurred to Copilot", "ErrorOccurred", "copilot-cli", "errorOccurred", true},
		// Gemini-only events
		{"BeforeModel to Gemini", "BeforeModel", "gemini-cli", "BeforeModel", true},
		{"AfterModel to Gemini", "AfterModel", "gemini-cli", "AfterModel", true},
		{"BeforeToolSelection to Gemini", "BeforeToolSelection", "gemini-cli", "BeforeToolSelection", true},
		// CC-only events have no targets
		{"PostToolUseFailure (CC-only)", "PostToolUseFailure", "gemini-cli", "PostToolUseFailure", false},
		{"Elicitation (CC-only)", "Elicitation", "cline", "Elicitation", false},
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
		// Cline events
		{"Cline TaskStart", "TaskStart", "cline", "SessionStart"},
		{"Cline TaskComplete", "TaskComplete", "cline", "SessionEnd"},
		{"Cline TaskResume", "TaskResume", "cline", "TaskResume"},
		// Copilot new events
		{"Copilot subagentStop", "subagentStop", "copilot-cli", "SubagentStop"},
		{"Copilot errorOccurred", "errorOccurred", "copilot-cli", "ErrorOccurred"},
		// Gemini-only events
		{"Gemini BeforeModel", "BeforeModel", "gemini-cli", "BeforeModel"},
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

func TestToolNamesKeyRename(t *testing.T) {
	// XX-1: "Task" was renamed to "Agent"
	if _, ok := ToolNames["Agent"]; !ok {
		t.Fatal("expected ToolNames to have 'Agent' key")
	}
	if _, ok := ToolNames["Task"]; ok {
		t.Fatal("expected ToolNames NOT to have 'Task' key (renamed to Agent)")
	}
}

func TestToolNamesWebFetch(t *testing.T) {
	// XX-2: WebFetch entry exists with correct provider mappings
	wf, ok := ToolNames["WebFetch"]
	if !ok {
		t.Fatal("expected ToolNames to have 'WebFetch' key")
	}
	expected := map[string]string{
		"gemini-cli":  "web_fetch",
		"copilot-cli": "web_fetch",
		"kiro":        "web_fetch",
		"opencode":    "webfetch",
		"zed":         "fetch",
		"windsurf":    "read_url_content",
	}
	for prov, want := range expected {
		got, exists := wf[prov]
		if !exists {
			t.Errorf("missing provider %q in WebFetch", prov)
		} else if got != want {
			t.Errorf("WebFetch[%q] = %q, want %q", prov, got, want)
		}
	}
	// Providers that should NOT have a mapping
	for _, prov := range []string{"cline", "roo-code", "cursor", "codex"} {
		if _, exists := wf[prov]; exists {
			t.Errorf("WebFetch should not have mapping for %q", prov)
		}
	}
}

func TestReverseTranslateTool_BackwardsCompat(t *testing.T) {
	// "task" (copilot-cli provider name) reverse-translates to "Agent"
	got := ReverseTranslateTool("task", "copilot-cli")
	assertEqual(t, "Agent", got)

	// "Task" (old canonical name) also maps to "Agent" via backwards compat
	got = ReverseTranslateTool("Task", "copilot-cli")
	assertEqual(t, "Agent", got)

	// "Task" with a provider that has no "task" mapping still maps to "Agent"
	got = ReverseTranslateTool("Task", "cline")
	assertEqual(t, "Agent", got)
}

func TestReverseTranslateTool_NewProviders(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		source string
		want   string
	}{
		// OpenCode
		{"OpenCode read", "read", "opencode", "Read"},
		{"OpenCode write", "write", "opencode", "Write"},
		{"OpenCode edit", "edit", "opencode", "Edit"},
		{"OpenCode bash", "bash", "opencode", "Bash"},
		{"OpenCode glob", "glob", "opencode", "Glob"},
		{"OpenCode grep", "grep", "opencode", "Grep"},
		{"OpenCode websearch", "websearch", "opencode", "WebSearch"},
		{"OpenCode task", "task", "opencode", "Agent"},
		// Cline
		{"Cline read_file", "read_file", "cline", "Read"},
		{"Cline write_to_file", "write_to_file", "cline", "Write"},
		{"Cline replace_in_file", "replace_in_file", "cline", "Edit"},
		{"Cline execute_command", "execute_command", "cline", "Bash"},
		{"Cline list_files", "list_files", "cline", "Glob"},
		{"Cline search_files", "search_files", "cline", "Grep"},
		// Roo Code
		{"RooCode read_file", "read_file", "roo-code", "Read"},
		{"RooCode write_to_file", "write_to_file", "roo-code", "Write"},
		{"RooCode replace_in_file", "replace_in_file", "roo-code", "Edit"},
		{"RooCode execute_command", "execute_command", "roo-code", "Bash"},
		{"RooCode list_files", "list_files", "roo-code", "Glob"},
		{"RooCode search_files", "search_files", "roo-code", "Grep"},
		// Zed
		{"Zed read_file", "read_file", "zed", "Read"},
		{"Zed terminal", "terminal", "zed", "Bash"},
		{"Zed find_path", "find_path", "zed", "Glob"},
		{"Zed grep", "grep", "zed", "Grep"},
		{"Zed web_search", "web_search", "zed", "WebSearch"},
		{"Zed spawn_agent", "spawn_agent", "zed", "Agent"},
		// Cursor
		{"Cursor read_file", "read_file", "cursor", "Read"},
		{"Cursor edit_file", "edit_file", "cursor", "Edit"},
		{"Cursor run_terminal_cmd", "run_terminal_cmd", "cursor", "Bash"},
		{"Cursor file_search", "file_search", "cursor", "Glob"},
		{"Cursor grep_search", "grep_search", "cursor", "Grep"},
		{"Cursor web_search", "web_search", "cursor", "WebSearch"},
		// Windsurf
		{"Windsurf view_line_range", "view_line_range", "windsurf", "Read"},
		{"Windsurf write_to_file", "write_to_file", "windsurf", "Write"},
		{"Windsurf edit_file", "edit_file", "windsurf", "Edit"},
		{"Windsurf run_command", "run_command", "windsurf", "Bash"},
		{"Windsurf find_by_name", "find_by_name", "windsurf", "Glob"},
		{"Windsurf grep_search", "grep_search", "windsurf", "Grep"},
		{"Windsurf search_web", "search_web", "windsurf", "WebSearch"},
		{"Windsurf read_url_content", "read_url_content", "windsurf", "WebFetch"},
		// Codex
		{"Codex read_file", "read_file", "codex", "Read"},
		{"Codex apply_patch", "apply_patch", "codex", "Edit"},
		{"Codex shell", "shell", "codex", "Bash"},
		{"Codex list_dir", "list_dir", "codex", "Glob"},
		{"Codex grep_files", "grep_files", "codex", "Grep"},
		{"Codex web_search", "web_search", "codex", "WebSearch"},
		{"Codex spawn_agent", "spawn_agent", "codex", "Agent"},
		// Copilot (fixed entries)
		{"Copilot view", "view", "copilot-cli", "Read"},
		{"Copilot create", "create", "copilot-cli", "Write"},
		{"Copilot edit", "edit", "copilot-cli", "Edit"},
		{"Copilot bash", "bash", "copilot-cli", "Bash"},
		{"Copilot grep", "grep", "copilot-cli", "Grep"},
		// Kiro (new entries)
		{"Kiro glob", "glob", "kiro", "Glob"},
		{"Kiro grep", "grep", "kiro", "Grep"},
		{"Kiro web_search", "web_search", "kiro", "WebSearch"},
		{"Kiro use_subagent", "use_subagent", "kiro", "Agent"},
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
		// Claude <-> various providers
		{"Claude to Gemini", "mcp__github__search_repositories", "claude-code", "gemini-cli", "mcp_github_search_repositories"},
		{"Claude to Copilot", "mcp__github__search_repositories", "claude-code", "copilot-cli", "github/search_repositories"},
		{"Gemini to Claude", "mcp_github_search_repositories", "gemini-cli", "claude-code", "mcp__github__search_repositories"},
		{"Copilot to Claude", "github/search_repositories", "copilot-cli", "claude-code", "mcp__github__search_repositories"},
		{"Not MCP tool", "regular_tool", "claude-code", "gemini-cli", "regular_tool"},
		// Bare double-underscore providers as source
		{"OpenCode to Claude", "github__search_repos", "opencode", "claude-code", "mcp__github__search_repos"},
		{"Cline to Claude", "github__search_repos", "cline", "claude-code", "mcp__github__search_repos"},
		{"RooCode to Claude", "github__search_repos", "roo-code", "claude-code", "mcp__github__search_repos"},
		{"Cursor to Claude", "github__search_repos", "cursor", "claude-code", "mcp__github__search_repos"},
		{"Windsurf to Claude", "github__search_repos", "windsurf", "claude-code", "mcp__github__search_repos"},
		// Zed colon format
		{"Zed to Claude", "mcp:github:search_repos", "zed", "claude-code", "mcp__github__search_repos"},
		{"Claude to Zed", "mcp__github__search_repos", "claude-code", "zed", "mcp:github:search_repos"},
		{"Zed to Gemini", "mcp:github:search_repos", "zed", "gemini-cli", "mcp_github_search_repos"},
		{"Gemini to Zed", "mcp_github_search_repos", "gemini-cli", "zed", "mcp:github:search_repos"},
		// Codex slash format
		{"Codex to Claude", "github/search_repos", "codex", "claude-code", "mcp__github__search_repos"},
		{"Claude to Codex", "mcp__github__search_repos", "claude-code", "codex", "github/search_repos"},
		// New providers as target
		{"Claude to OpenCode", "mcp__github__search_repos", "claude-code", "opencode", "github__search_repos"},
		{"Claude to Cline", "mcp__github__search_repos", "claude-code", "cline", "github__search_repos"},
		{"Claude to RooCode", "mcp__github__search_repos", "claude-code", "roo-code", "github__search_repos"},
		{"Claude to Cursor", "mcp__github__search_repos", "claude-code", "cursor", "github__search_repos"},
		{"Claude to Windsurf", "mcp__github__search_repos", "claude-code", "windsurf", "github__search_repos"},
		// Cross-provider
		{"Gemini to OpenCode", "mcp_github_search_repos", "gemini-cli", "opencode", "github__search_repos"},
		{"Copilot to Zed", "github/search_repos", "copilot-cli", "zed", "mcp:github:search_repos"},
		{"OpenCode to Zed", "github__search_repos", "opencode", "zed", "mcp:github:search_repos"},
		{"Zed to OpenCode", "mcp:github:search_repos", "zed", "opencode", "github__search_repos"},
		// Non-MCP tool names should pass through
		{"Zed non-MCP", "regular_tool", "zed", "claude-code", "regular_tool"},
		{"Gemini non-MCP", "regular_tool", "gemini-cli", "claude-code", "regular_tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateMCPToolName(tt.tool, tt.source, tt.target)
			assertEqual(t, tt.want, got)
		})
	}
}
