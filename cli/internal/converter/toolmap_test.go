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
		{"file_read to Gemini", "file_read", "gemini-cli", "read_file"},
		{"file_read to Copilot", "file_read", "copilot-cli", "view"},
		{"shell to Gemini", "shell", "gemini-cli", "run_shell_command"},
		{"Unknown tool passes through", "CustomTool", "gemini-cli", "CustomTool"},
		{"web_search to Gemini", "web_search", "gemini-cli", "google_web_search"},
		{"web_search to Copilot (no mapping)", "web_search", "copilot-cli", "web_search"},
		{"Claude target translates", "file_read", "claude-code", "Read"},
		// OpenCode
		{"file_read to OpenCode", "file_read", "opencode", "read"},
		{"file_write to OpenCode", "file_write", "opencode", "write"},
		{"file_edit to OpenCode", "file_edit", "opencode", "edit"},
		{"shell to OpenCode", "shell", "opencode", "bash"},
		{"find to OpenCode", "find", "opencode", "glob"},
		{"search to OpenCode", "search", "opencode", "grep"},
		{"web_search to OpenCode", "web_search", "opencode", "websearch"},
		{"agent to OpenCode", "agent", "opencode", "task"},
		// Zed
		{"file_read to Zed", "file_read", "zed", "read_file"},
		{"file_write to Zed", "file_write", "zed", "edit_file"},
		{"file_edit to Zed", "file_edit", "zed", "edit_file"},
		{"shell to Zed", "shell", "zed", "terminal"},
		{"find to Zed", "find", "zed", "find_path"},
		{"search to Zed", "search", "zed", "grep"},
		{"web_search to Zed", "web_search", "zed", "web_search"},
		{"agent to Zed", "agent", "zed", "spawn_agent"},
		// Cline
		{"file_read to Cline", "file_read", "cline", "read_file"},
		{"file_write to Cline", "file_write", "cline", "write_to_file"},
		{"file_edit to Cline", "file_edit", "cline", "replace_in_file"},
		{"shell to Cline", "shell", "cline", "execute_command"},
		{"find to Cline", "find", "cline", "list_files"},
		{"search to Cline", "search", "cline", "search_files"},
		{"web_search to Cline (no mapping)", "web_search", "cline", "web_search"},
		{"agent to Cline (no mapping)", "agent", "cline", "agent"},
		// Roo Code
		{"file_read to RooCode", "file_read", "roo-code", "read_file"},
		{"file_write to RooCode", "file_write", "roo-code", "write_to_file"},
		{"file_edit to RooCode", "file_edit", "roo-code", "replace_in_file"},
		{"shell to RooCode", "shell", "roo-code", "execute_command"},
		{"find to RooCode", "find", "roo-code", "list_files"},
		{"search to RooCode", "search", "roo-code", "search_files"},
		{"web_search to RooCode (no mapping)", "web_search", "roo-code", "web_search"},
		{"agent to RooCode (no mapping)", "agent", "roo-code", "agent"},
		// Cursor
		{"file_read to Cursor", "file_read", "cursor", "read_file"},
		{"file_write to Cursor", "file_write", "cursor", "edit_file"},
		{"file_edit to Cursor", "file_edit", "cursor", "edit_file"},
		{"shell to Cursor", "shell", "cursor", "run_terminal_cmd"},
		{"find to Cursor", "find", "cursor", "file_search"},
		{"search to Cursor", "search", "cursor", "grep_search"},
		{"web_search to Cursor", "web_search", "cursor", "web_search"},
		// Windsurf
		{"file_read to Windsurf", "file_read", "windsurf", "view_line_range"},
		{"file_write to Windsurf", "file_write", "windsurf", "write_to_file"},
		{"file_edit to Windsurf", "file_edit", "windsurf", "edit_file"},
		{"shell to Windsurf", "shell", "windsurf", "run_command"},
		{"find to Windsurf", "find", "windsurf", "find_by_name"},
		{"search to Windsurf", "search", "windsurf", "grep_search"},
		{"web_search to Windsurf", "web_search", "windsurf", "search_web"},
		{"web_fetch to Windsurf", "web_fetch", "windsurf", "read_url_content"},
		// Codex
		{"file_read to Codex", "file_read", "codex", "read_file"},
		{"file_write to Codex", "file_write", "codex", "apply_patch"},
		{"file_edit to Codex", "file_edit", "codex", "apply_patch"},
		{"shell to Codex", "shell", "codex", "shell"},
		{"find to Codex", "find", "codex", "list_dir"},
		{"search to Codex", "search", "codex", "grep_files"},
		{"web_search to Codex", "web_search", "codex", "web_search"},
		{"agent to Codex", "agent", "codex", "spawn_agent"},
		// Copilot (fixed entries)
		{"file_write to Copilot", "file_write", "copilot-cli", "create"},
		{"file_edit to Copilot", "file_edit", "copilot-cli", "edit"},
		{"search to Copilot", "search", "copilot-cli", "grep"},
		{"shell to Copilot", "shell", "copilot-cli", "bash"},
		// Kiro (new entries)
		{"find to Kiro", "find", "kiro", "glob"},
		{"search to Kiro", "search", "kiro", "grep"},
		{"web_search to Kiro", "web_search", "kiro", "web_search"},
		{"agent to Kiro", "agent", "kiro", "use_subagent"},
		// web_fetch
		{"web_fetch to Gemini", "web_fetch", "gemini-cli", "web_fetch"},
		{"web_fetch to Copilot", "web_fetch", "copilot-cli", "web_fetch"},
		{"web_fetch to Kiro", "web_fetch", "kiro", "web_fetch"},
		{"web_fetch to OpenCode", "web_fetch", "opencode", "webfetch"},
		{"web_fetch to Zed", "web_fetch", "zed", "fetch"},
		{"web_fetch to Cline (no mapping)", "web_fetch", "cline", "web_fetch"},
		{"web_fetch to RooCode (no mapping)", "web_fetch", "roo-code", "web_fetch"},
		{"web_fetch to Cursor (no mapping)", "web_fetch", "cursor", "web_fetch"},
		{"web_fetch to Codex (no mapping)", "web_fetch", "codex", "web_fetch"},
		// CC-only tools pass through when target has no mapping
		{"notebook_edit to Gemini (no mapping)", "notebook_edit", "gemini-cli", "notebook_edit"},
		{"multi_edit to Gemini (no mapping)", "multi_edit", "gemini-cli", "multi_edit"},
		{"list_dir to Gemini (no mapping)", "list_dir", "gemini-cli", "list_dir"},
		{"notebook_read to Gemini (no mapping)", "notebook_read", "gemini-cli", "notebook_read"},
		{"kill_shell to Gemini (no mapping)", "kill_shell", "gemini-cli", "kill_shell"},
		{"multi_edit to Cline (no mapping)", "multi_edit", "cline", "multi_edit"},
		{"list_dir to Cursor (no mapping)", "list_dir", "cursor", "list_dir"},
		{"notebook_read to Windsurf (no mapping)", "notebook_read", "windsurf", "notebook_read"},
		{"kill_shell to Codex (no mapping)", "kill_shell", "codex", "kill_shell"},
		{"skill to Gemini (no mapping)", "skill", "gemini-cli", "skill"},
		{"ask_user to Gemini (no mapping)", "ask_user", "gemini-cli", "ask_user"},
		// CC-only tools translate to CC native names
		{"notebook_edit to CC", "notebook_edit", "claude-code", "NotebookEdit"},
		{"multi_edit to CC", "multi_edit", "claude-code", "MultiEdit"},
		{"list_dir to CC", "list_dir", "claude-code", "LS"},
		{"kill_shell to CC", "kill_shell", "claude-code", "KillBash"},
		{"skill to CC", "skill", "claude-code", "Skill"},
		{"ask_user to CC", "ask_user", "claude-code", "AskUserQuestion"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateTool(tt.tool, tt.target)
			assertEqual(t, tt.wantResult, got)
		})
	}
}

func TestTranslateTools(t *testing.T) {
	input := []string{"file_read", "file_write", "shell"}
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
		{"before_tool_execute to Gemini", "before_tool_execute", "gemini-cli", "BeforeTool", true},
		{"before_tool_execute to Copilot", "before_tool_execute", "copilot-cli", "preToolUse", true},
		{"before_tool_execute to CC", "before_tool_execute", "claude-code", "PreToolUse", true},
		{"agent_stop to CC", "agent_stop", "claude-code", "Stop", true},
		{"agent_stop to Copilot", "agent_stop", "copilot-cli", "agentStop", true},
		{"agent_stop to Gemini", "agent_stop", "gemini-cli", "AfterAgent", true},
		{"subagent_start (no targets except CC)", "subagent_start", "gemini-cli", "subagent_start", false},
		{"Unknown event", "FakeEvent", "gemini-cli", "FakeEvent", false},
		// Cline hook events
		{"before_tool_execute to Cline", "before_tool_execute", "cline", "PreToolUse", true},
		{"after_tool_execute to Cline", "after_tool_execute", "cline", "PostToolUse", true},
		{"session_start to Cline", "session_start", "cline", "TaskStart", true},
		{"session_end to Cline", "session_end", "cline", "TaskComplete", true},
		{"before_compact to Cline", "before_compact", "cline", "PreCompact", true},
		// Cline-only events
		{"task_resume to Cline", "task_resume", "cline", "TaskResume", true},
		{"task_cancel to Cline", "task_cancel", "cline", "TaskCancel", true},
		// Copilot new events
		{"subagent_stop to Copilot", "subagent_stop", "copilot-cli", "subagentStop", true},
		{"error_occurred to Copilot", "error_occurred", "copilot-cli", "errorOccurred", true},
		// Gemini-only events
		{"before_model to Gemini", "before_model", "gemini-cli", "BeforeModel", true},
		{"after_model to Gemini", "after_model", "gemini-cli", "AfterModel", true},
		{"before_tool_selection to Gemini", "before_tool_selection", "gemini-cli", "BeforeToolSelection", true},
		// Gemini-only events unsupported by other providers
		{"before_model to Copilot (unsupported)", "before_model", "copilot-cli", "before_model", false},
		{"after_model to Copilot (unsupported)", "after_model", "copilot-cli", "after_model", false},
		{"before_tool_selection to Copilot (unsupported)", "before_tool_selection", "copilot-cli", "before_tool_selection", false},
		{"before_model to Kiro (unsupported)", "before_model", "kiro", "before_model", false},
		{"after_model to Cline (unsupported)", "after_model", "cline", "after_model", false},
		// CC-only events have no targets except CC
		{"elicitation (CC-only) to Cline", "elicitation", "cline", "elicitation", false},
		// CC-only events translate to CC
		{"elicitation to CC", "elicitation", "claude-code", "Elicitation", true},
		// tool_use_failure (renamed from after_tool_failure)
		{"tool_use_failure to CC", "tool_use_failure", "claude-code", "PostToolUseFailure", true},
		{"tool_use_failure to Cursor", "tool_use_failure", "cursor", "postToolUseFailure", true},
		{"tool_use_failure to Copilot", "tool_use_failure", "copilot-cli", "errorOccurred", true},
		{"tool_use_failure to Gemini (unsupported)", "tool_use_failure", "gemini-cli", "tool_use_failure", false},
		// Windsurf events
		{"session_start to Windsurf", "session_start", "windsurf", "session_start", true},
		{"session_end to Windsurf", "session_end", "windsurf", "session_end", true},
		{"before_prompt to Windsurf", "before_prompt", "windsurf", "pre_user_prompt", true},
		{"agent_stop to Windsurf", "agent_stop", "windsurf", "post_cascade_response", true},
		// Opencode events
		{"before_tool_execute to Opencode", "before_tool_execute", "opencode", "tool.execute.before", true},
		{"after_tool_execute to Opencode", "after_tool_execute", "opencode", "tool.execute.after", true},
		{"session_start to Opencode", "session_start", "opencode", "session.created", true},
		{"agent_stop to Opencode", "agent_stop", "opencode", "session.idle", true},
		{"error_occurred to Opencode", "error_occurred", "opencode", "session.error", true},
		{"permission_request to Opencode", "permission_request", "opencode", "permission.asked", true},
		// Cursor extended events
		{"subagent_start to Cursor", "subagent_start", "cursor", "SubagentStart", true},
		{"subagent_stop to Cursor", "subagent_stop", "cursor", "SubagentStop", true},
		{"before_model to Cursor", "before_model", "cursor", "beforeAgentResponse", true},
		{"after_model to Cursor", "after_model", "cursor", "afterAgentResponse", true},
		{"before_tool_selection to Cursor", "before_tool_selection", "cursor", "beforeToolSelection", true},
		// New events
		{"file_changed to CC", "file_changed", "claude-code", "FileChanged", true},
		{"file_changed to Cursor", "file_changed", "cursor", "afterFileEdit", true},
		{"file_changed to Kiro", "file_changed", "kiro", "File Save", true},
		{"file_changed to Opencode", "file_changed", "opencode", "file.edited", true},
		{"file_created to Kiro", "file_created", "kiro", "File Create", true},
		{"file_deleted to Kiro", "file_deleted", "kiro", "File Delete", true},
		{"before_task to Kiro", "before_task", "kiro", "Pre Task Execution", true},
		{"after_task to Kiro", "after_task", "kiro", "Post Task Execution", true},
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
		{"Gemini BeforeTool", "BeforeTool", "gemini-cli", "before_tool_execute"},
		{"Copilot preToolUse", "preToolUse", "copilot-cli", "before_tool_execute"},
		{"CC PreToolUse", "PreToolUse", "claude-code", "before_tool_execute"},
		{"CC Stop", "Stop", "claude-code", "agent_stop"},
		{"Unknown event", "FakeEvent", "gemini-cli", "FakeEvent"},
		// Cline events
		{"Cline TaskStart", "TaskStart", "cline", "session_start"},
		{"Cline TaskComplete", "TaskComplete", "cline", "session_end"},
		{"Cline TaskResume", "TaskResume", "cline", "task_resume"},
		// Copilot new events
		{"Copilot subagentStop", "subagentStop", "copilot-cli", "subagent_stop"},
		{"Copilot agentStop", "agentStop", "copilot-cli", "agent_stop"},
		// Note: errorOccurred maps to both error_occurred and tool_use_failure;
		// test moved to TestReverseTranslateHookEvent_CopilotAmbiguous below.
		// Gemini-only events
		{"Gemini BeforeModel", "BeforeModel", "gemini-cli", "before_model"},
		{"Gemini AfterModel", "AfterModel", "gemini-cli", "after_model"},
		{"Gemini BeforeToolSelection", "BeforeToolSelection", "gemini-cli", "before_tool_selection"},
		// Opencode reverse
		{"Opencode tool.execute.before", "tool.execute.before", "opencode", "before_tool_execute"},
		{"Opencode session.created", "session.created", "opencode", "session_start"},
		// Windsurf reverse
		{"Windsurf pre_user_prompt", "pre_user_prompt", "windsurf", "before_prompt"},
		{"Windsurf post_cascade_response", "post_cascade_response", "windsurf", "agent_stop"},
		// Cursor extended reverse
		{"Cursor beforeAgentResponse", "beforeAgentResponse", "cursor", "before_model"},
		{"Cursor afterAgentResponse", "afterAgentResponse", "cursor", "after_model"},
		// Kiro new events reverse
		{"Kiro File Save", "File Save", "kiro", "file_changed"},
		{"Kiro File Create", "File Create", "kiro", "file_created"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateHookEvent(tt.event, tt.source)
			assertEqual(t, tt.want, got)
		})
	}
}

func TestReverseTranslateHookEvent_CopilotAmbiguous(t *testing.T) {
	// Copilot's "errorOccurred" maps to both error_occurred and tool_use_failure.
	// Go map iteration order is non-deterministic, so either is valid.
	got := ReverseTranslateHookEvent("errorOccurred", "copilot-cli")
	if got != "error_occurred" && got != "tool_use_failure" {
		t.Errorf("expected error_occurred or tool_use_failure, got %q", got)
	}
}

func TestReverseTranslateTool(t *testing.T) {
	got := ReverseTranslateTool("read_file", "gemini-cli")
	assertEqual(t, "file_read", got)

	got = ReverseTranslateTool("unknown_tool", "gemini-cli")
	assertEqual(t, "unknown_tool", got)
}

func TestToolNamesKeyRename(t *testing.T) {
	// "agent" is the neutral canonical key (CC's "Agent" tool renamed from "Task")
	if _, ok := ToolNames["agent"]; !ok {
		t.Fatal("expected ToolNames to have 'agent' key")
	}
	if _, ok := ToolNames["Task"]; ok {
		t.Fatal("expected ToolNames NOT to have 'Task' key")
	}
	if _, ok := ToolNames["Agent"]; ok {
		t.Fatal("expected ToolNames NOT to have 'Agent' key (should be 'agent')")
	}
}

func TestToolNamesWebFetch(t *testing.T) {
	wf, ok := ToolNames["web_fetch"]
	if !ok {
		t.Fatal("expected ToolNames to have 'web_fetch' key")
	}
	expected := map[string]string{
		"claude-code": "WebFetch",
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
			t.Errorf("missing provider %q in web_fetch", prov)
		} else if got != want {
			t.Errorf("web_fetch[%q] = %q, want %q", prov, got, want)
		}
	}
	// Providers that should NOT have a mapping
	for _, prov := range []string{"cline", "roo-code", "cursor", "codex"} {
		if _, exists := wf[prov]; exists {
			t.Errorf("web_fetch should not have mapping for %q", prov)
		}
	}
}

func TestReverseTranslateTool_BackwardsCompat(t *testing.T) {
	// "task" (copilot-cli provider name) reverse-translates to "agent"
	got := ReverseTranslateTool("task", "copilot-cli")
	assertEqual(t, "agent", got)

	// CC's legacy "Task" tool name maps to "agent" via backwards compat
	got = ReverseTranslateTool("Task", "claude-code")
	assertEqual(t, "agent", got)

	// "task" (case-insensitive) from CC also maps to "agent"
	got = ReverseTranslateTool("task", "claude-code")
	assertEqual(t, "agent", got)
}

func TestReverseTranslateTool_CCToNeutral(t *testing.T) {
	// CC tool names now reverse-translate to neutral canonical names
	tests := []struct {
		name string
		tool string
		want string
	}{
		{"CC Read", "Read", "file_read"},
		{"CC Write", "Write", "file_write"},
		{"CC Edit", "Edit", "file_edit"},
		{"CC Bash", "Bash", "shell"},
		{"CC Glob", "Glob", "find"},
		{"CC Grep", "Grep", "search"},
		{"CC WebSearch", "WebSearch", "web_search"},
		{"CC WebFetch", "WebFetch", "web_fetch"},
		{"CC Agent", "Agent", "agent"},
		{"CC NotebookEdit", "NotebookEdit", "notebook_edit"},
		{"CC MultiEdit", "MultiEdit", "multi_edit"},
		{"CC LS", "LS", "list_dir"},
		{"CC KillBash", "KillBash", "kill_shell"},
		{"CC Skill", "Skill", "skill"},
		{"CC AskUserQuestion", "AskUserQuestion", "ask_user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateTool(tt.tool, "claude-code")
			assertEqual(t, tt.want, got)
		})
	}
}

func TestReverseTranslateTool_NewProviders(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		source string
		want   string
	}{
		// OpenCode
		{"OpenCode read", "read", "opencode", "file_read"},
		{"OpenCode write", "write", "opencode", "file_write"},
		{"OpenCode edit", "edit", "opencode", "file_edit"},
		{"OpenCode bash", "bash", "opencode", "shell"},
		{"OpenCode glob", "glob", "opencode", "find"},
		{"OpenCode grep", "grep", "opencode", "search"},
		{"OpenCode websearch", "websearch", "opencode", "web_search"},
		{"OpenCode task", "task", "opencode", "agent"},
		// Cline
		{"Cline read_file", "read_file", "cline", "file_read"},
		{"Cline write_to_file", "write_to_file", "cline", "file_write"},
		{"Cline replace_in_file", "replace_in_file", "cline", "file_edit"},
		{"Cline execute_command", "execute_command", "cline", "shell"},
		{"Cline list_files", "list_files", "cline", "find"},
		{"Cline search_files", "search_files", "cline", "search"},
		// Roo Code
		{"RooCode read_file", "read_file", "roo-code", "file_read"},
		{"RooCode write_to_file", "write_to_file", "roo-code", "file_write"},
		{"RooCode replace_in_file", "replace_in_file", "roo-code", "file_edit"},
		{"RooCode execute_command", "execute_command", "roo-code", "shell"},
		{"RooCode list_files", "list_files", "roo-code", "find"},
		{"RooCode search_files", "search_files", "roo-code", "search"},
		// Zed
		{"Zed read_file", "read_file", "zed", "file_read"},
		{"Zed terminal", "terminal", "zed", "shell"},
		{"Zed find_path", "find_path", "zed", "find"},
		{"Zed grep", "grep", "zed", "search"},
		{"Zed web_search", "web_search", "zed", "web_search"},
		{"Zed spawn_agent", "spawn_agent", "zed", "agent"},
		// Cursor
		{"Cursor read_file", "read_file", "cursor", "file_read"},
		{"Cursor edit_file", "edit_file", "cursor", "file_edit"},
		{"Cursor run_terminal_cmd", "run_terminal_cmd", "cursor", "shell"},
		{"Cursor file_search", "file_search", "cursor", "find"},
		{"Cursor grep_search", "grep_search", "cursor", "search"},
		{"Cursor web_search", "web_search", "cursor", "web_search"},
		// Windsurf
		{"Windsurf view_line_range", "view_line_range", "windsurf", "file_read"},
		{"Windsurf write_to_file", "write_to_file", "windsurf", "file_write"},
		{"Windsurf edit_file", "edit_file", "windsurf", "file_edit"},
		{"Windsurf run_command", "run_command", "windsurf", "shell"},
		{"Windsurf find_by_name", "find_by_name", "windsurf", "find"},
		{"Windsurf grep_search", "grep_search", "windsurf", "search"},
		{"Windsurf search_web", "search_web", "windsurf", "web_search"},
		{"Windsurf read_url_content", "read_url_content", "windsurf", "web_fetch"},
		// Codex
		{"Codex read_file", "read_file", "codex", "file_read"},
		{"Codex apply_patch", "apply_patch", "codex", "file_edit"},
		{"Codex shell", "shell", "codex", "shell"},
		{"Codex list_dir", "list_dir", "codex", "find"},
		{"Codex grep_files", "grep_files", "codex", "search"},
		{"Codex web_search", "web_search", "codex", "web_search"},
		{"Codex spawn_agent", "spawn_agent", "codex", "agent"},
		// Copilot (fixed entries)
		{"Copilot view", "view", "copilot-cli", "file_read"},
		{"Copilot create", "create", "copilot-cli", "file_write"},
		{"Copilot edit", "edit", "copilot-cli", "file_edit"},
		{"Copilot bash", "bash", "copilot-cli", "shell"},
		{"Copilot grep", "grep", "copilot-cli", "search"},
		// Kiro (new entries)
		{"Kiro glob", "glob", "kiro", "find"},
		{"Kiro grep", "grep", "kiro", "search"},
		{"Kiro web_search", "web_search", "kiro", "web_search"},
		{"Kiro use_subagent", "use_subagent", "kiro", "agent"},
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

func TestTranslateMatcher(t *testing.T) {
	tests := []struct {
		name   string
		match  string
		target string
		want   string
	}{
		// Simple tool name — works like TranslateTool
		{"simple shell to Gemini", "shell", "gemini-cli", "run_shell_command"},
		{"simple file_edit to Copilot", "file_edit", "copilot-cli", "edit"},
		{"unknown tool passes through", "CustomTool", "gemini-cli", "CustomTool"},

		// Regex alternation — each component translated individually
		{"file_edit|file_write to Gemini", "file_edit|file_write", "gemini-cli", "replace|write_file"},
		{"file_edit|file_write to Copilot", "file_edit|file_write", "copilot-cli", "edit|create"},
		{"three components", "file_edit|file_write|shell", "gemini-cli", "replace|write_file|run_shell_command"},

		// Wildcard suffix preserved
		{"shell.* to Gemini", "shell.*", "gemini-cli", "run_shell_command.*"},

		// MCP prefix patterns pass through unchanged
		{"mcp__github__.* unchanged", "mcp__github__.*", "gemini-cli", "mcp__github__.*"},
		{"mcp__github__create_issue unchanged", "mcp__github__create_issue", "gemini-cli", "mcp__github__create_issue"},

		// Bare wildcard passes through
		{".* passes through", ".*", "gemini-cli", ".*"},

		// Mixed: alternation with MCP pattern
		{"file_edit|mcp__github__.*", "file_edit|mcp__github__.*", "gemini-cli", "replace|mcp__github__.*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TranslateMatcher(tt.match, tt.target)
			assertEqual(t, tt.want, got)
		})
	}
}

func TestReverseTranslateMatcher(t *testing.T) {
	tests := []struct {
		name   string
		match  string
		source string
		want   string
	}{
		// Simple reverse
		{"simple Gemini read_file", "read_file", "gemini-cli", "file_read"},

		// Regex alternation
		{"replace|write_file from Gemini", "replace|write_file", "gemini-cli", "file_edit|file_write"},
		{"three components from Gemini", "replace|write_file|run_shell_command", "gemini-cli", "file_edit|file_write|shell"},

		// Wildcard suffix preserved
		{"run_shell_command.* from Gemini", "run_shell_command.*", "gemini-cli", "shell.*"},

		// MCP prefix patterns pass through unchanged
		{"mcp__github__.* unchanged", "mcp__github__.*", "gemini-cli", "mcp__github__.*"},

		// Bare wildcard
		{".* passes through", ".*", "gemini-cli", ".*"},

		// CC matchers reverse to neutral
		{"CC Bash to neutral", "Bash", "claude-code", "shell"},
		{"CC Edit|Write to neutral", "Edit|Write", "claude-code", "file_edit|file_write"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReverseTranslateMatcher(tt.match, tt.source)
			assertEqual(t, tt.want, got)
		})
	}
}

func TestIsValidHookEvent(t *testing.T) {
	tests := []struct {
		event string
		want  bool
	}{
		// Canonical names (map keys)
		{"before_tool_execute", true},
		{"after_tool_execute", true},
		{"session_start", true},
		{"agent_stop", true},
		{"notification", true},
		{"elicitation", true},
		// Provider-native names (map values)
		{"PreToolUse", true},
		{"PostToolUse", true},
		{"BeforeTool", true},
		{"AfterTool", true},
		{"UserPromptSubmit", true},
		{"Stop", true},
		{"SessionStart", true},
		{"TaskStart", true},  // Cline
		{"TaskResume", true}, // Cline
		// New events and provider-native names
		{"file_changed", true},
		{"file_created", true},
		{"tool_use_failure", true},
		{"tool.execute.before", true}, // Opencode
		{"pre_user_prompt", true},     // Windsurf
		{"File Save", true},           // Kiro
		// Invalid names
		{"", false},
		{"FakeEvent", false},
		{"PreToolUse.evil.path", false}, // sjson key injection attempt
		{"hooks.PreToolUse", false},     // dotted path
		{"../../../etc/passwd", false},  // path traversal
	}
	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := IsValidHookEvent(tt.event)
			if got != tt.want {
				t.Errorf("IsValidHookEvent(%q) = %v, want %v", tt.event, got, tt.want)
			}
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
